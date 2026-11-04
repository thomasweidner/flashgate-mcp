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
	"sort"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

var updateMCPSnapshots = flag.Bool("update-mcp-snapshots", false, "explicitly replace reviewed MCP wire snapshots")

// Each entry is executable functional coverage, independent of snapshot updates.
// A new registration requires an explicit success and security case here.
type mcpToolCase struct {
	arguments string
	denied    string
	field     string
	want      any
	readOnly  bool
}

func mcpToolCases() map[string]mcpToolCase {
	return map[string]mcpToolCase{
		"list_directory":   {`{}`, `{"path":"../outside"}`, "entries", []any{map[string]any{"name": "seed.txt", "isDir": false, "size": json.Number("4")}}, true},
		"read_file":        {`{"path":"seed.txt"}`, `{"path":"../outside"}`, "content", "seed", true},
		"get_path_info":    {`{"path":"seed.txt"}`, `{"path":"../outside"}`, "exists", true, true},
		"write_file":       {`{"path":"written.txt","content":"new"}`, `{"path":"../outside","content":"new"}`, "written", true, false},
		"create_directory": {`{"path":"new-directory"}`, `{"path":"../outside"}`, "created", true, false},
		"delete_path":      {`{"path":"seed.txt"}`, `{"path":"../outside"}`, "deleted", true, false},
		"copy_path":        {`{"source":"seed.txt","target":"copied.txt"}`, `{"source":"../outside","target":"copied.txt"}`, "copied", true, false},
		"move_path":        {`{"source":"seed.txt","target":"moved.txt"}`, `{"source":"../outside","target":"moved.txt"}`, "moved", true, false},
	}
}

func mcpRegistry(t *testing.T, readOnly bool) (*tools.Registry, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	return createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(readOnly)), root
}

func mcpRouter(registry *tools.Registry, candidate bool) *router.Router {
	if !candidate {
		return createRouter("flashgate", "conformance-fixture", registry)
	}
	r := router.NewWithPolicy("flashgate", "conformance-fixture", revision.NewPolicy(protocol.ProtocolVersion, revision.Stateless))
	r.Register(initialize.NewHandler("flashgate", "conformance-fixture"))
	r.Register(tools.NewListHandler(registry))
	r.Register(tools.NewCallHandler(registry))
	return r
}

// Exercise the production JSON-RPC transport, parser, dispatcher and serializers.
// Notifications are deliberately counted separately from request responses.
func mcpWire(t *testing.T, r *router.Router, input string, count int) []map[string]json.RawMessage {
	t.Helper()
	var stdout bytes.Buffer
	if err := mcpserver.New(strings.NewReader(input), &stdout, r).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if count == 0 && stdout.Len() == 0 {
		return nil
	}
	if len(lines) != count {
		t.Fatalf("stdout purity/response count: got %q, want %d JSON-RPC lines", stdout.String(), count)
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

func mcpRequest(method, params, version string) string {
	var values map[string]any
	if err := json.Unmarshal([]byte(params), &values); err != nil {
		panic(err)
	}
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
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
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
		if test.arguments == "" || test.denied == "" || test.field == "" {
			return fmt.Errorf("missing executable contract case for %q", definition.Name)
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

func TestMCPConformanceSnapshots(t *testing.T) {
	for _, version := range []string{protocol.ProtocolVersion, revision.Stateless} {
		for _, profile := range []string{"default", "read-only"} {
			t.Run(version+"/"+profile, func(t *testing.T) {
				readOnly := profile == "read-only"
				registry, root := mcpRegistry(t, readOnly)
				r := mcpRouter(registry, version == revision.Stateless)
				definitions := make([]protocol.Tool, 0)
				for _, tool := range registry.List() {
					definitions = append(definitions, tool.Definition())
				}
				sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
				if err := mcpInventory(definitions, mcpToolCases(), readOnly); err != nil {
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
				if err := json.Unmarshal(list["tools"], &listed); err != nil {
					t.Fatal(err)
				}
				if err := mcpInventory(listed, mcpToolCases(), readOnly); err != nil {
					t.Fatal(err)
				}
				sort.Slice(listed, func(i, j int) bool { return listed[i].Name < listed[j].Name })
				if !bytes.Equal(canonicalMCP(t, definitions), canonicalMCP(t, listed)) {
					t.Fatal("wire catalog differs from sorted runtime definitions")
				}
				results := make(map[string]any)
				for _, definition := range definitions {
					test := mcpToolCases()[definition.Name]
					outcomes := make(map[string]any)
					for _, outcome := range []string{"success", "denied"} {
						fresh, _ := mcpRegistry(t, readOnly)
						arguments := test.arguments
						if outcome == "denied" {
							arguments = test.denied
						}
						result := mcpResult(t, mcpRouter(fresh, version == revision.Stateless), "tools/call", `{"name":"`+definition.Name+`","arguments":`+arguments+`}`, version)
						mcpCheckRevisionResult(t, result, version, false)
						outcomes[outcome] = result
						// Strip only validated adapter fields before reusing the strict
						// legacy CallToolResult decoder; domain contracts stay shared.
						base := make(map[string]json.RawMessage)
						for key, value := range result {
							if key != "_meta" && key != "resultType" {
								base[key] = value
							}
						}
						decoded, err := mcptest.DecodeCallToolResult(canonicalMCP(t, base))
						if err != nil || decoded.IsError != (outcome == "denied") {
							t.Fatalf("%s/%s result contract: %v", definition.Name, outcome, err)
						}
						if outcome == "success" {
							value, ok := decoded.StructuredContent.(map[string]any)
							if !ok || !reflect.DeepEqual(value[test.field], test.want) {
								t.Fatalf("%s: functional assertion %s=%v failed: %#v", definition.Name, test.field, test.want, value)
							}
						}
					}
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
	definitions := []protocol.Tool{}
	for _, tool := range registry.List() {
		definitions = append(definitions, tool.Definition())
	}
	if err := mcpInventory(definitions, mcpToolCases(), false); err != nil {
		t.Fatal(err)
	}
	t.Run("new-tool-without-contract", func(t *testing.T) {
		base, ok := registry.Get("read_file")
		if !ok {
			t.Fatal("missing mutation seed")
		}
		registry.Register(mcpFutureTool{Tool: base})
		mutated := []protocol.Tool{}
		for _, tool := range registry.List() {
			mutated = append(mutated, tool.Definition())
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
	t.Run("profile-drift", func(t *testing.T) {
		if mcpInventory(definitions, mcpToolCases(), true) == nil {
			t.Fatal("write tools escaped read-only profile gate")
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
	for _, version := range []string{protocol.ProtocolVersion, revision.Stateless} {
		for _, profile := range []string{"default", "read-only"} {
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
