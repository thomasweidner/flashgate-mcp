package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/ping"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

var updateMCPSnapshots = flag.Bool("update-mcp-snapshots", false, "explicitly replace reviewed MCP wire snapshots")

// Effective profiles added by BL-110 must extend this shared snapshot matrix.
var mcpConformanceVersions = []string{protocol.ProtocolVersion, revision.Stateless}
var mcpConformanceProfiles = []string{"default", "read-only"}

func mcpCandidatePolicy() revision.Policy {
	return revision.NewPolicy(append(revision.ProductionPolicy().Supported(), revision.Stateless)...)
}

func mcpMatrixCoverage(versions, profiles []string) error {
	policyVersions := mcpCandidatePolicy().Supported()
	if !slices.Equal(slices.Sorted(slices.Values(versions)), policyVersions) {
		return fmt.Errorf("snapshot revisions %v must cover production and candidate policy revisions %v exactly once", versions, policyVersions)
	}
	// These are the two tool catalogs selected by filesystem.read_only. The owner
	// of effective profiles must bind that replacement selector here.
	if !slices.Equal(slices.Sorted(slices.Values(profiles)), []string{"default", "read-only"}) {
		return fmt.Errorf("snapshot profiles %v must cover both startup modes exactly once", profiles)
	}
	return nil
}

// Each entry is executable functional coverage, independent of snapshot updates.
// A new registration requires an explicit success and security case here.
type mcpToolCase struct {
	success  map[string]mcpSuccessCase
	denied   string
	readOnly bool
	errors   map[string]mcpErrorCase
}

type mcpSuccessCase struct {
	arguments string
	field     string
	want      any
	content   []protocol.ContentBlock
}

func mcpCompleteCase(arguments, denied, field string, errors map[string]mcpErrorCase) mcpToolCase {
	return mcpToolCase{success: map[string]mcpSuccessCase{"complete": {arguments: arguments, field: field, want: true}}, denied: denied, errors: errors}
}

func mcpToolCases() map[string]mcpToolCase {
	return map[string]mcpToolCase{
		"list_directory": {success: map[string]mcpSuccessCase{
			"complete": {`{}`, "entries", []any{map[string]any{"name": "seed.txt", "isDir": false, "size": json.Number("4")}}, []protocol.ContentBlock{protocol.NewTextContent("See structuredContent for the result.")}},
		}, denied: `{"path":"../outside"}`, readOnly: true, errors: mcpFilesystemErrors(`{}`, map[string]mcpErrorCase{
			"not_found":             {arguments: `{"path":"missing.txt"}`},
			"already_exists":        {reason: "Listing does not create or replace a path."},
			"unsupported_path_type": {reason: "Listing uses native directory enumeration. A non-directory enumeration error has no portable BL-203 path-type mapping here."},
			"limit_exceeded":        {arguments: `{}`, setup: wireSecondFile, tightLimits: true},
		})},
		"read_file": {success: map[string]mcpSuccessCase{
			"complete": {`{"path":"seed.txt"}`, "size", json.Number("4"), []protocol.ContentBlock{protocol.NewTextContent("seed")}},
		}, denied: `{"path":"../outside"}`, readOnly: true, errors: mcpFilesystemErrors(`{"path":"seed.txt"}`, map[string]mcpErrorCase{
			"not_found":             {arguments: `{"path":"missing.txt"}`},
			"already_exists":        {reason: "Reading does not create or replace a path."},
			"unsupported_path_type": {arguments: `{"path":"directory"}`, setup: wireDirectory},
			"invalid_text_encoding": {arguments: `{"path":"invalid.txt"}`, setup: wireInvalidText, copiedBytes: uint64(len(wireInvalidTextBytes))},
			"limit_exceeded":        {arguments: `{"path":"seed.txt","maxBytes":3}`},
		})},
		"get_path_info": {success: map[string]mcpSuccessCase{
			"existing": {arguments: `{"path":"seed.txt"}`, field: "exists", want: true},
			"missing":  {arguments: `{"path":"missing.txt"}`, field: "exists", want: false},
		}, denied: `{"path":"../outside"}`, readOnly: true, errors: mcpFilesystemErrors(`{"path":"seed.txt"}`, map[string]mcpErrorCase{
			"not_found":             {reason: "A missing path is the existing successful missing variant."},
			"already_exists":        {reason: "Stat does not create or replace a path."},
			"unsupported_path_type": {reason: "Stat supports files and directories. PathGuard denials use invalid_path."},
			"limit_exceeded":        {reason: "Stat does not read content, traverse a collection or accept a variable-size result projection."},
		})},
		"write_file": mcpCompleteCase(`{"path":"written.txt","content":"new"}`, `{"path":"../outside","content":"new"}`, "written",
			mcpFilesystemErrors(`{"path":"written.txt","content":"new"}`, map[string]mcpErrorCase{
				"not_found":             {arguments: `{"path":"missing/file.txt"}`},
				"already_exists":        {arguments: `{"path":"seed.txt"}`},
				"unsupported_path_type": {arguments: `{"path":"directory"}`, setup: wireDirectory},
				"limit_exceeded":        {arguments: `{"path":"written.txt","content":"too large"}`, tightLimits: true},
			})),
		"create_directory": mcpCompleteCase(`{"path":"new-directory"}`, `{"path":"../outside"}`, "created",
			mcpFilesystemErrors(`{"path":"new-directory"}`, map[string]mcpErrorCase{
				"not_found":             {reason: "Mkdir creates missing ancestors and leaf."},
				"already_exists":        {reason: "An existing directory returns created:false; an existing file is a path-type error."},
				"unsupported_path_type": {arguments: `{"path":"seed.txt"}`},
				"limit_exceeded":        {reason: "Mkdir has no filesystem traversal, content or collection quota for its receipt."},
			})),
		"delete_path": mcpCompleteCase(`{"path":"seed.txt"}`, `{"path":"../outside"}`, "deleted",
			mcpFilesystemErrors(`{"path":"seed.txt"}`, map[string]mcpErrorCase{
				"not_found":             {arguments: `{"path":"missing.txt"}`},
				"already_exists":        {reason: "Deletion requires an existing source and creates no destination."},
				"unsupported_path_type": {arguments: `{"path":"directory"}`, setup: wireNonemptyDirectory},
				"limit_exceeded":        {arguments: `{"path":"directory","recursive":true}`, setup: wireNonemptyDirectory, tightLimits: true},
			})),
		"copy_path": mcpCompleteCase(`{"source":"seed.txt","target":"copied.txt"}`, `{"source":"../outside","target":"copied.txt"}`, "copied",
			mcpFilesystemErrors(`{"source":"seed.txt","target":"copied.txt"}`, map[string]mcpErrorCase{
				"not_found":             {arguments: `{"source":"missing.txt","target":"copied.txt"}`},
				"already_exists":        {arguments: `{"source":"seed.txt","target":"second.txt"}`, setup: wireSecondFile},
				"unsupported_path_type": {arguments: `{"source":"directory","target":"copied.txt"}`, setup: wireDirectory},
				"limit_exceeded":        {arguments: `{"source":"seed.txt","target":"copied.txt"}`, tightLimits: true},
			})),
		"move_path": mcpCompleteCase(`{"source":"seed.txt","target":"moved.txt"}`, `{"source":"../outside","target":"moved.txt"}`, "moved",
			mcpFilesystemErrors(`{"source":"seed.txt","target":"moved.txt"}`, map[string]mcpErrorCase{
				"not_found":             {arguments: `{"source":"missing.txt","target":"moved.txt"}`},
				"already_exists":        {arguments: `{"source":"seed.txt","target":"second.txt"}`, setup: wireSecondFile},
				"unsupported_path_type": {arguments: `{"source":"seed.txt","target":"directory","overwrite":true}`, setup: wireDirectory},
				"unsupported_operation": {arguments: `{"source":"seed.txt","target":"moved.txt"}`, backendError: fs.ErrCrossVolumeMoveUnsupported},
				"limit_exceeded":        {reason: "Same-volume rename does not traverse or transfer content and has no operation quota."},
			})),
	}
}

func mcpVariantCoverage(contract tools.ResultContract, cases map[string]mcpSuccessCase) error {
	if len(contract) != len(cases) {
		return fmt.Errorf("declared variants %d differ from executable fixtures %d", len(contract), len(cases))
	}
	for _, variant := range contract {
		fixture, ok := cases[variant.Name]
		if !ok || fixture.arguments == "" || fixture.field == "" {
			return fmt.Errorf("missing executable fixture for variant %q", variant.Name)
		}
		if (variant.ObjectKind != tools.MetadataPayload || len(variant.Parts) > 0) && len(fixture.content) == 0 {
			return fmt.Errorf("missing exact payload block assertions for variant %q", variant.Name)
		}
	}
	return nil
}

type mcpObservedTool struct {
	tools.Tool
	variant string
}

func (tool *mcpObservedTool) Execute(ctx context.Context, arguments json.RawMessage) (tools.ToolResult, error) {
	result, err := tool.Tool.Execute(ctx, arguments)
	tool.variant = result.Variant()
	return result, err
}

func mcpRegistry(t *testing.T, readOnly bool) (*tools.Registry, string) {
	t.Helper()
	return mcpRegistryWithLimit(t, readOnly, 1024)
}

func mcpRegistryWithLimit(t *testing.T, readOnly bool, maxFileBytes int64) (*tools.Registry, string) {
	t.Helper()
	return mcpWireRegistry(t, readOnly, maxFileBytes, fs.DefaultLimits(), nil)
}

func mcpWireRegistry(t *testing.T, readOnly bool, maxFileBytes int64, limits fs.Limits, wrap func(fs.FileSystem) fs.FileSystem) (*tools.Registry, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	local, err := fs.NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), limits)
	if err != nil {
		t.Fatal(err)
	}
	var filesystem fs.FileSystem = local
	if wrap != nil {
		filesystem = wrap(local)
	}
	return requireToolRegistry(t, filesystem, maxFileBytes, capabilitiesFromReadOnly(readOnly)), root
}

func mcpRouter(registry *tools.Registry, candidate bool) *router.Router {
	if !candidate {
		return createRouter("flashgate", "conformance-fixture", registry)
	}
	r := router.NewWithPolicy("flashgate", "conformance-fixture", mcpCandidatePolicy())
	r.Register(initialize.NewHandler("flashgate", "conformance-fixture"))
	r.Register(ping.Handler{})
	r.Register(tools.NewListHandler(registry))
	r.Register(tools.NewCallHandler(registry))
	return r
}

// Exercise the production JSON-RPC transport, parser, dispatcher and serializers.
// Notifications are deliberately counted separately from request responses.
func mcpWire(t *testing.T, r *router.Router, input string, count int) []map[string]json.RawMessage {
	t.Helper()
	stdout, err := mcpStdout(context.Background(), r, input)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if count == 0 && len(stdout) == 0 {
		return nil
	}
	if len(lines) != count {
		t.Fatalf("stdout purity/response count: got %q, want %d JSON-RPC lines", stdout, count)
	}
	results := make([]map[string]json.RawMessage, 0, count)
	for _, line := range lines {
		var response map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &response); err != nil || string(response["jsonrpc"]) != `"2.0"` || response["id"] == nil {
			t.Fatalf("non-protocol stdout: %q", line)
		}
		if (response["result"] == nil) == (response["error"] == nil) || len(response) != 3 {
			t.Fatalf("invalid response envelope: %q", line)
		}
		results = append(results, response)
	}
	return results
}

func mcpStdout(ctx context.Context, r *router.Router, input string) ([]byte, error) {
	var stdout bytes.Buffer
	err := mcpserver.New(strings.NewReader(input), &stdout, r).Run(ctx)
	return stdout.Bytes(), err
}

func mcpRequest(method, params, version string) string {
	value, err := mcptest.SchemaInstance([]byte(params))
	if err != nil {
		panic(err)
	}
	values := value.(map[string]any)
	if version == revision.Stateless {
		values["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    version,
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		}
	}
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": values})
	if err != nil {
		panic(err)
	}
	return string(request) + "\n"
}

func mcpResult(t *testing.T, r *router.Router, method, params, version string) map[string]json.RawMessage {
	t.Helper()
	response := mcpWire(t, r, mcpRequest(method, params, version), 1)[0]
	if response["error"] != nil {
		t.Fatalf("%s: unexpected RPC error: %s", method, response["error"])
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(response["result"], &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func canonicalMCP(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := mcptest.SchemaInstance(encoded)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func mcpInventory(definitions []protocol.Tool, cases map[string]mcpToolCase, readOnly bool) error {
	expected := make(map[string]bool)
	for name, test := range cases {
		if !readOnly || test.readOnly {
			expected[name] = true
		}
	}
	for _, definition := range definitions {
		if !expected[definition.Name] {
			return fmt.Errorf("uncovered, duplicate or incorrectly profiled tool %q", definition.Name)
		}
		delete(expected, definition.Name)
		test := cases[definition.Name]
		if len(test.success) == 0 || test.denied == "" {
			return fmt.Errorf("missing executable contract case for %q", definition.Name)
		}
		if err := mcpErrorCoverage(test.errors); err != nil {
			return fmt.Errorf("%s: %w", definition.Name, err)
		}
	}
	if len(expected) != 0 {
		return fmt.Errorf("missing expected tools: %v", expected)
	}
	return nil
}

func mcpCapabilities(raw json.RawMessage) error {
	var capabilities map[string]json.RawMessage
	if err := json.Unmarshal(raw, &capabilities); err != nil {
		return err
	}
	if len(capabilities) != 1 || string(capabilities["tools"]) != `{}` {
		return fmt.Errorf("capability lacks an explicit test mapping: %s", raw)
	}
	return nil
}

func mcpRevisions(production, discovered []string) error {
	if !reflect.DeepEqual(production, []string{"2025-11-25"}) || !reflect.DeepEqual(discovered, []string{"2026-07-28"}) {
		return fmt.Errorf("revision-specific coverage required: production=%v candidate=%v", production, discovered)
	}
	return nil
}

func mcpAnnotations(raw []byte) error {
	var definition map[string]json.RawMessage
	if err := json.Unmarshal(raw, &definition); err != nil {
		return err
	}
	var annotations map[string]json.RawMessage
	if err := json.Unmarshal(definition["annotations"], &annotations); err != nil {
		return err
	}
	if len(annotations) != 4 {
		return fmt.Errorf("annotation contract changed: %s", raw)
	}
	for _, key := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
		if value := string(annotations[key]); value != "true" && value != "false" {
			return fmt.Errorf("missing explicit boolean annotation %q", key)
		}
	}
	return nil
}

func mcpSnapshotDrift(accepted, current []byte) error {
	if !bytes.Equal(accepted, current) {
		return fmt.Errorf("unaccepted MCP snapshot drift")
	}
	return nil
}

func mcpDefinitions(registry *tools.Registry) []protocol.Tool {
	var definitions []protocol.Tool
	for _, tool := range registry.List() {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

func TestMCPConformanceSnapshots(t *testing.T) {
	if err := mcpMatrixCoverage(mcpConformanceVersions, mcpConformanceProfiles); err != nil {
		t.Fatal(err)
	}
	cases := mcpToolCases()
	for _, version := range mcpConformanceVersions {
		for _, profile := range mcpConformanceProfiles {
			t.Run(version+"/"+profile, func(t *testing.T) {
				readOnly := profile == "read-only"
				registry, root := mcpRegistry(t, readOnly)
				r := mcpRouter(registry, version == revision.Stateless)
				definitions := mcpDefinitions(registry)
				schemas, err := mcpCompileSchemas(definitions)
				if err != nil {
					t.Fatal(err)
				}
				if profile == "default" {
					mcpCheckCatalog(t, definitions)
				}
				sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
				if err := mcpInventory(definitions, cases, readOnly); err != nil {
					t.Fatal(err)
				}
				for _, definition := range definitions {
					if err := mcpAnnotations(canonicalMCP(t, definition)); err != nil {
						t.Fatal(err)
					}
				}
				method, params := "initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"contract","version":"1"}}`
				if version == revision.Stateless {
					method, params = "server/discover", `{}`
				}
				discovery := mcpResult(t, r, method, params, version)
				if err := mcpCapabilities(discovery["capabilities"]); err != nil {
					t.Fatal(err)
				}
				if version == revision.Stateless {
					var versions []string
					if err := json.Unmarshal(discovery["supportedVersions"], &versions); err != nil {
						t.Fatal(err)
					}
					if err := mcpRevisions(revision.ProductionPolicy().Supported(), versions); err != nil {
						t.Fatal(err)
					}
				} else if string(discovery["protocolVersion"]) != `"2025-11-25"` {
					t.Fatal("legacy advertised revision changed without coverage")
				}
				list := mcpResult(t, r, "tools/list", `{}`, version)
				var listed []protocol.Tool
				decoder := json.NewDecoder(bytes.NewReader(list["tools"]))
				decoder.UseNumber()
				if err := decoder.Decode(&listed); err != nil {
					t.Fatal(err)
				}
				if err := mcpInventory(listed, cases, readOnly); err != nil {
					t.Fatal(err)
				}
				sort.Slice(listed, func(i, j int) bool { return listed[i].Name < listed[j].Name })
				if !bytes.Equal(canonicalMCP(t, definitions), canonicalMCP(t, listed)) {
					t.Fatal("wire catalog differs from sorted runtime definitions")
				}
				results := make(map[string]any)
				for _, definition := range definitions {
					test := cases[definition.Name]
					schema := schemas[definition.Name]
					registered, ok := registry.Get(definition.Name)
					if !ok {
						t.Fatalf("missing registered tool %q", definition.Name)
					}
					contract := registered.ResultContract()
					if err := mcpVariantCoverage(contract, test.success); err != nil {
						t.Fatalf("%s: %v", definition.Name, err)
					}
					outcomes := make(map[string]any)
					for variant, fixture := range test.success {
						fresh, _ := mcpRegistry(t, readOnly)
						original, _ := fresh.Get(definition.Name)
						observed := &mcpObservedTool{Tool: original}
						if err := fresh.Register(observed); err != nil {
							t.Fatal(err)
						}
						label := definition.Name + "/" + variant
						mcpRequireSchemaInstance(t, label+"/inputSchema", schema.input, []byte(fixture.arguments))
						result := mcpResult(t, mcpRouter(fresh, version == revision.Stateless), "tools/call", `{"name":"`+definition.Name+`","arguments":`+fixture.arguments+`}`, version)
						mcpCheckRevisionResult(t, result, version, false)
						if observed.variant != variant {
							t.Fatalf("%s: executed variant %q", label, observed.variant)
						}
						mcpRequireSchemaInstance(t, label+"/outputSchema", schema.output, result["structuredContent"])
						base := mcpResultEnvelope(t, result)
						decoded, err := mcpDecodeSuccess(t, base, fixture)
						if err != nil || decoded.IsError {
							t.Fatalf("%s: invalid success envelope: %v, %s", label, err, base)
						}
						value := decoded.StructuredContent.(map[string]any)
						if !reflect.DeepEqual(value[fixture.field], fixture.want) {
							t.Fatalf("%s: functional assertion failed: %s", label, base)
						}
						outcome := "success_" + variant
						if variant == contract[0].Name {
							outcome = "success"
						}
						outcomes[outcome] = result
					}
					fresh, _ := mcpRegistry(t, readOnly)
					mcpRequireSchemaInstance(t, definition.Name+"/denied/inputSchema", schema.input, []byte(test.denied))
					denied := mcpResult(t, mcpRouter(fresh, version == revision.Stateless), "tools/call", `{"name":"`+definition.Name+`","arguments":`+test.denied+`}`, version)
					mcpCheckRevisionResult(t, denied, version, false)
					if decoded, err := mcptest.DecodeCallToolResult(mcpResultEnvelope(t, denied)); err != nil || !decoded.IsError {
						t.Fatalf("%s: invalid denial: %v", definition.Name, err)
					}
					outcomes["denied"] = denied
					results[definition.Name] = outcomes
				}
				mcpCheckRevisionResult(t, list, version, true)
				mcpCheckRevisionResult(t, discovery, version, version == revision.Stateless)
				snapshot := canonicalMCP(t, map[string]any{
					"format": "flashgate-mcp-contract/v1", "revision": version, "profile": profile,
					"productionRevisions": revision.ProductionPolicy().Supported(),
					"definitions":         definitions, "toolsList": list, "entry": discovery, "calls": results,
				})
				if bytes.Contains(snapshot, []byte(root)) {
					t.Fatal("host root leaked into snapshot")
				}
				path := filepath.Join("testdata", "mcp", version+"-"+profile+".json")
				if *updateMCPSnapshots {
					if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, snapshot, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				accepted, err := os.ReadFile(path)
				if err != nil || mcpSnapshotDrift(accepted, snapshot) != nil {
					t.Fatalf("unaccepted MCP drift in %s (explicit -update-mcp-snapshots and review required): %v", path, err)
				}
			})
		}
	}
}

func mcpDecodeSuccess(t *testing.T, raw json.RawMessage, fixture mcpSuccessCase) (mcptest.DecodedCallToolResult, error) {
	t.Helper()
	if fixture.content == nil {
		return mcptest.DecodeCallToolResult(raw)
	}
	return mcptest.DecodeDeclaredCallToolResult(raw, canonicalMCP(t, fixture.content))
}

func mcpResultEnvelope(t *testing.T, result map[string]json.RawMessage) json.RawMessage {
	t.Helper()
	base := make(map[string]json.RawMessage, len(result))
	for key, value := range result {
		if key != "_meta" && key != "resultType" {
			base[key] = value
		}
	}
	return canonicalMCP(t, base)
}

func mcpCheckRevisionResult(t *testing.T, result map[string]json.RawMessage, version string, cacheable bool) {
	t.Helper()
	if version != revision.Stateless {
		for _, key := range []string{"_meta", "resultType", "ttlMs", "cacheScope"} {
			if result[key] != nil {
				t.Fatalf("modern field %s leaked into legacy result", key)
			}
		}
		return
	}
	if string(result["resultType"]) != `"complete"` {
		t.Fatal("missing complete resultType")
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(result["_meta"], &meta); err != nil || len(meta) != 1 {
		t.Fatal("invalid revision-bound result metadata")
	}
	var identity map[string]string
	if err := json.Unmarshal(meta["io.modelcontextprotocol/serverInfo"], &identity); err != nil || len(identity) != 2 || identity["name"] != "flashgate" || identity["version"] != "conformance-fixture" {
		t.Fatal("invalid serverInfo")
	}
	if cacheable {
		if string(result["ttlMs"]) != "0" || string(result["cacheScope"]) != `"private"` {
			t.Fatal("missing conservative revision cache hints")
		}
	} else if result["ttlMs"] != nil || result["cacheScope"] != nil {
		t.Fatal("cache hints on noncacheable result")
	}
}

func TestMCPConformanceCompletenessMutations(t *testing.T) {
	registry, _ := mcpRegistry(t, false)
	definitions := mcpDefinitions(registry)
	if err := mcpInventory(definitions, mcpToolCases(), false); err != nil {
		t.Fatal(err)
	}
	t.Run("new-tool-without-contract", func(t *testing.T) {
		base, ok := registry.Get("read_file")
		if !ok {
			t.Fatal("missing mutation seed")
		}
		if err := registry.Register(mcpFutureTool{Tool: base}); err != nil {
			t.Fatal(err)
		}
		mutated := mcpDefinitions(registry)
		schemas, err := mcpCompileSchemas(mutated)
		if err != nil || schemas["future_tool"].input == nil || schemas["future_tool"].output == nil {
			t.Fatalf("synthetic registration was not compiled: %v", err)
		}
		if mcpInventory(mutated, mcpToolCases(), false) == nil {
			t.Fatal("new tool escaped contract completeness gate")
		}
		if mcpSnapshotDrift(canonicalMCP(t, definitions), canonicalMCP(t, mutated)) == nil {
			t.Fatal("new registry tool escaped snapshot gate")
		}
	})
	t.Run("missing-contract", func(t *testing.T) {
		cases := mcpToolCases()
		cases["read_file"] = mcpToolCase{readOnly: true}
		if mcpInventory(definitions, cases, false) == nil {
			t.Fatal("empty contract escaped gate")
		}
	})
	t.Run("missing-variant-fixture", func(t *testing.T) {
		tool, _ := registry.Get("get_path_info")
		cases := mcpToolCases()["get_path_info"].success
		delete(cases, "missing")
		if mcpVariantCoverage(tool.ResultContract(), cases) == nil {
			t.Fatal("untested result variant escaped the conformance gate")
		}
	})
	t.Run("missing-error-applicability", func(t *testing.T) {
		cases := mcpToolCases()
		delete(cases["get_path_info"].errors, "not_found")
		if err := mcpInventory(definitions, cases, false); err == nil || !strings.Contains(err.Error(), "missing error applicability") {
			t.Fatalf("missing applicability escaped the conformance gate: %v", err)
		}
	})
	t.Run("missing-applicable-error-fixture", func(t *testing.T) {
		cases := mcpToolCases()
		cases["read_file"].errors["not_found"] = mcpErrorCase{}
		if err := mcpInventory(definitions, cases, false); err == nil || !strings.Contains(err.Error(), "missing applicable error fixture") {
			t.Fatalf("missing error fixture escaped the conformance gate: %v", err)
		}
	})
	t.Run("duplicated-payload", func(t *testing.T) {
		fixture := mcpToolCases()["read_file"].success["complete"]
		raw := json.RawMessage(`{"content":[{"type":"text","text":"seed"},{"type":"text","text":"seed"}],"structuredContent":{"size":4}}`)
		if _, err := mcpDecodeSuccess(t, raw, fixture); err == nil {
			t.Fatal("duplicate payload escaped the conformance gate")
		}
	})
	t.Run("profile-drift", func(t *testing.T) {
		if mcpInventory(definitions, mcpToolCases(), true) == nil {
			t.Fatal("write tools escaped read-only profile gate")
		}
	})
	t.Run("incomplete-matrix", func(t *testing.T) {
		for _, test := range []struct {
			versions, profiles []string
		}{
			{[]string{protocol.ProtocolVersion}, mcpConformanceProfiles},
			{[]string{revision.Stateless}, mcpConformanceProfiles},
			{[]string{protocol.ProtocolVersion, "future"}, mcpConformanceProfiles},
			{mcpConformanceVersions, []string{"default"}},
			{mcpConformanceVersions, []string{"read-only"}},
			{mcpConformanceVersions, []string{"default", "default"}},
			{mcpConformanceVersions, []string{"default", "unknown"}},
		} {
			if mcpMatrixCoverage(test.versions, test.profiles) == nil {
				t.Fatalf("incomplete revision/profile matrix accepted: %v / %v", test.versions, test.profiles)
			}
		}
	})
	t.Run("missing-tool", func(t *testing.T) {
		if mcpInventory(definitions[1:], mcpToolCases(), false) == nil {
			t.Fatal("missing tool escaped gate")
		}
	})
	t.Run("unknown-capability", func(t *testing.T) {
		if mcpCapabilities(json.RawMessage(`{"tools":{},"resources":{}}`)) == nil || mcpCapabilities(json.RawMessage(`{"tools":{"listChanged":true}}`)) == nil {
			t.Fatal("unmapped capability escaped gate")
		}
	})
	t.Run("activated-or-announced-revision", func(t *testing.T) {
		if mcpRevisions([]string{protocol.ProtocolVersion, revision.Stateless}, []string{revision.Stateless}) == nil || mcpRevisions([]string{protocol.ProtocolVersion}, []string{revision.Stateless, "future"}) == nil {
			t.Fatal("unreviewed revision escaped gate")
		}
	})
	t.Run("missing-explicit-false", func(t *testing.T) {
		raw := canonicalMCP(t, definitions[0])
		mutated := bytes.Replace(raw, []byte(`"openWorldHint": false`), []byte(`"unknownHint": false`), 1)
		if bytes.Equal(raw, mutated) || mcpAnnotations(mutated) == nil {
			t.Fatal("missing annotation escaped gate")
		}
	})
	for _, field := range []string{"inputSchema", "outputSchema", "name", "title", "description", "annotations"} {
		t.Run("snapshot-drift-"+field, func(t *testing.T) {
			var value map[string]any
			accepted := canonicalMCP(t, definitions[0])
			if err := json.Unmarshal(accepted, &value); err != nil {
				t.Fatal(err)
			}
			value[field] = "mutation"
			if mcpSnapshotDrift(accepted, canonicalMCP(t, value)) == nil {
				t.Fatal("mutation escaped exact snapshot comparison")
			}
		})
	}
}

type mcpFutureTool struct{ tools.Tool }

func (mcpFutureTool) Name() string { return "future_tool" }
func (tool mcpFutureTool) Definition() protocol.Tool {
	definition := tool.Tool.Definition()
	definition.Name = tool.Name()
	return definition
}

func TestMCPConformanceInventory(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "mcp", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"official-requirements.json": true}
	for _, version := range mcpConformanceVersions {
		for _, profile := range mcpConformanceProfiles {
			expected[version+"-"+profile+".json"] = true
		}
	}
	for _, path := range paths {
		if !expected[filepath.Base(path)] {
			t.Fatalf("snapshot has no revision/profile coverage: %s", path)
		}
		delete(expected, filepath.Base(path))
	}
	if len(expected) != 0 {
		t.Fatalf("missing snapshots: %v", expected)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "mcp", "official-requirements.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mapping struct {
		UpstreamCommit string `json:"upstreamCommit"`
		Scenarios      []struct {
			Revision       string `json:"revision"`
			Scenario       string `json:"scenario"`
			Scoring        string `json:"scoring"`
			Applicability  string `json:"applicability"`
			OfficialStatus string `json:"officialStatus"`
			NativeReason   string `json:"nativeReason"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(data, &mapping); err != nil {
		t.Fatal(err)
	}
	if mapping.UpstreamCommit != "c37eec888e1c6ff140af79987a40008548b7cc5f" || len(mapping.Scenarios) != 83 {
		t.Fatal("pinned official requirement inventory changed")
	}
	seen := make(map[string]bool)
	for _, row := range mapping.Scenarios {
		key := row.Revision + "/" + row.Scenario
		if seen[key] || (row.Revision != protocol.ProtocolVersion && row.Revision != revision.Stateless) || row.Scenario == "" || row.NativeReason == "" || row.OfficialStatus != "NOT_RUN" || (row.Scoring != "scored" && row.Scoring != "not_scored") {
			t.Fatalf("unmapped/invalid official requirement: %s", key)
		}
		switch row.Applicability {
		case "APPLICABLE", "APPLICABLE_SUBSET", "NOT_APPLICABLE", "DEFERRED":
		default:
			t.Fatalf("missing applicability: %s", key)
		}
		seen[key] = true
	}
}
