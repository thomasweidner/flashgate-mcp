package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

type outcomeFixtureFS struct{ fs.FileSystem }

func (outcomeFixtureFS) LimitsForRoot(config.RootID) (resource.Limits, error) {
	return fs.DefaultLimits().ResourceLimits(), nil
}

func (outcomeFixtureFS) Stat(path fs.Operand) (fs.Metadata, error) {
	if path.Path == "absent.txt" {
		return fs.Metadata{}, fs.ErrNotFound
	}
	return fs.Metadata{Name: "fixture"}, nil
}

type outcomeFixtureBackend struct{}

func (outcomeFixtureBackend) Execute(_ context.Context, call execution.Invocation[execution.OperationKey]) (execution.Completion[bool], error) {
	return execution.Complete(call.Binding(), true), nil
}

func TestMCPConformanceConditionalOperationOutcomes(t *testing.T) {
	profile := decodeTestConfig(t, "").Profile()
	run, err := execution.NewRuntime("outcome-fixture")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := execution.NewPolicyBinding(run, profile)
	if err != nil {
		t.Fatal(err)
	}
	session, identity, err := execution.BindDirectSTDIO(run)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.WithPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	base := execution.OperationKey{Name: "fixture.lookup", Variant: execution.DefaultVariant}
	optional := execution.OperationKey{Name: base.Name, Variant: "optional"}
	operation, err := profile.Catalog().NewOperation(capability.RiskRead, capability.FilesRead)
	if err != nil {
		t.Fatal(err)
	}
	keys := []execution.OperationKey{base, optional}
	dispatch, err := execution.NewDispatcher(policy, execution.CurrentProcess, execution.Routes[execution.OperationKey]{
		Declarations: []execution.Declaration{{Key: base, Operation: operation}, {Key: optional, Operation: operation}},
		Key:          func(key execution.OperationKey) execution.OperationKey { return key },
		Resolve:      func(context.Context, execution.OperationKey) ([]execution.Resource, error) { return nil, nil },
	}, execution.Registration[execution.OperationKey, bool]{
		ID: execution.CurrentProcess, Identity: identity, Operations: keys, Backend: outcomeFixtureBackend{},
	})
	if err != nil {
		t.Fatal(err)
	}
	required := []tools.CatalogOutcome{
		{Operation: base, Result: "existing"}, {Operation: optional, Result: "existing"}, {Operation: optional, Result: "missing"},
	}
	metadata := tools.NewGetPathInfoTool(outcomeFixtureFS{})
	representation := tools.CatalogRepresentation{
		Support: tools.CatalogSupport{
			Revision: execution.MCP20251125, Backend: execution.CurrentProcess, Origin: execution.InheritedSTDIO, Platform: runtime.GOOS,
		},
		Metadata: metadata, Variants: keys, Outcomes: required,
	}
	makeCase := func(key execution.OperationKey, result, fixture, path string) mcpOperationCase {
		return mcpOperationCase{key: key, variant: result, fixture: fixture, success: mcpSuccessCase{
			arguments: `{"root_id":"work","path":"` + path + `"}`, field: "exists", want: result == "existing",
		}}
	}
	cases := []mcpOperationCase{
		makeCase(base, "existing", "reference", "base.txt"),
		makeCase(optional, "existing", "optional", "option.txt"),
		makeCase(optional, "missing", "optional", "absent.txt"),
		makeCase(optional, "existing", "repeated", "repeated.txt"),
	}
	contract := metadata.ResultContract()
	if err := mcpFixtureCoverage(contract, cases, required); err != nil {
		t.Fatal(err)
	}
	bind := func(ctx handlers.Context) (execution.CatalogContext, error) {
		request, err := session.Request(execution.Protocol(ctx.ProtocolVersion), 4096)
		if err != nil {
			return execution.CatalogContext{}, err
		}
		return dispatch.CatalogContext(request)
	}
	master, err := tools.NewCatalog(bind, func(view *tools.CatalogView) error {
		var rows []wireOperationCase
		for _, fixture := range cases {
			rows = append(rows, fixture.binding())
		}
		return checkOperationCases(view.Outcomes(metadata.Name()), rows, func(wireOperationCase) error { return nil })
	}, &tools.CatalogTool{
		Representations: []tools.CatalogRepresentation{representation},
		Handle: func(ctx handlers.Context, invocation execution.CatalogInvocation, args json.RawMessage) (tools.ToolResult, error) {
			key := optional
			if bytes.Contains(args, []byte("base.txt")) {
				key = base
			}
			if _, err := dispatch.Dispatch(ctx.Context, invocation.Request(), key, nil); err != nil {
				return tools.ToolResult{}, err
			}
			return metadata.Execute(ctx.Context, args)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := admitProfile(master); err != nil {
		t.Fatal(err)
	}
	var budgets activationBudgets
	if err := json.Unmarshal([]byte(wireBudgetManifest), &budgets); err != nil {
		t.Fatal(err)
	}
	budgetName := func(row wireOperationCase) string {
		return "tools/call/get_path_info/" + row.Variant + "/" + row.Fixture
	}
	var measured []wireOperationCase
	for _, fixture := range cases {
		var observed mcpObservedExecution
		result := mcpResultContext(t, observed.context(t.Context()), mcpRouter(master, false), "tools/call",
			`{"name":"get_path_info","arguments":`+fixture.success.arguments+`}`, protocol.ProtocolVersion)
		if err := observed.check(fixture.key, fixture.variant, true); err != nil {
			t.Fatal(err)
		}
		otherKey := base
		if fixture.key == base {
			otherKey = optional
		}
		if observed.check(otherKey, fixture.variant, true) == nil || observed.check(fixture.key, "different", true) == nil {
			t.Fatal("mismatched operation or result was accepted with the same wire bytes")
		}
		row := fixture.binding()
		row.Response = canonicalMCP(t, result)
		measured = append(measured, row)
		budgets.Cases[budgetName(row)] = budgets.Cases["tools/call/get_path_info/"+row.Variant+"/reference"]
	}
	verifyBudget := func(row wireOperationCase) error {
		index := slices.IndexFunc(contract, func(variant tools.ResultVariant) bool { return variant.Name == row.Variant })
		return budgets.check("production/"+protocol.ProtocolVersion+"/read/stdio-lf", budgetName(row), wireVariantBinding(contract[index]), row.Response)
	}
	if err := checkOperationCases(required, measured, verifyBudget); err != nil {
		t.Fatal(err)
	}
	wrong := slices.Clone(measured)
	wrong[2].Operation = wireOperationKey(base)
	for _, rows := range [][]wireOperationCase{measured[:2], wrong, append(slices.Clone(measured), measured[0])} {
		if checkOperationCases(required, rows, verifyBudget) == nil {
			t.Fatal("incomplete, wrong or duplicate outcomes were accepted")
		}
	}
	delete(budgets.Cases, budgetName(measured[2]))
	if checkOperationCases(required, measured, verifyBudget) == nil {
		t.Fatal("an observed outcome without its budget was accepted")
	}
}

func TestEffectiveProfilesOnSTDIO(t *testing.T) {
	for _, tc := range []struct {
		name, settings string
		writable       bool
	}{
		{"default", "[profiles.future]\ngrants = ['search.run', 'system.read']\n", false},
		{"explicit read ceiling", "[profiles]\nmax_risk = 'read'\n", false},
		{"builtin write", profileSettings(false), true},
		{"custom write", "[profiles]\nactive = 'dev'\n[profiles.dev]\ngrants = ['files.write', 'files.read']\n", true},
		{"composed write", "[profiles]\nactive = 'read'\ninclude = ['changes']\n[profiles.changes]\ngrants = ['files.write']\n", true},
		{"tightened write", "[profiles]\nactive = 'write'\nmax_risk = 'read'\n", false},
		{"high ceiling with default grants", "[profiles]\nmax_risk = 'high'\n", false},
		{"high ceiling with write grants", "[profiles]\nactive = 'write'\nmax_risk = 'high'\n", true},
	} {
		for _, multiple := range []bool{false, true} {
			variant := "one root"
			if multiple {
				variant = "multiple roots"
			}
			t.Run(tc.name+"/"+variant, func(t *testing.T) {
				root := t.TempDir()
				settings := "work.access = ['files.read', 'files.write', 'search.run']\n" + tc.settings
				if multiple {
					settings = "second.path = " + quotedConfigPath(t, t.TempDir()) + "\n" + settings
				}
				path := filepath.Join(root, "flashgate.toml")
				document := configDocument(t, root, settings)
				testconfig.Write(t, path, []byte(document))
				testconfig.Write(t, filepath.Join(root, "ordinary.txt"), []byte("ordinary sibling"))
				input := strings.Join([]string{
					`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
					`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_file","arguments":{"root_id":"work","path":"ordinary.txt"}}}`,
					`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"root_id":"work","path":"flashgate.toml"}}}`,
					`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"root_id":"work","path":"created.txt","content":"new"}}}`,
					`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"list_processes","arguments":{}}}`,
					`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"stop_external_process","arguments":{}}}`,
				}, "\n") + "\n"
				var output, diagnostics bytes.Buffer
				if err := runWithIO(context.Background(), strings.NewReader(input), &output, &diagnostics, defaultBootstrapDependencies(path)); err != nil {
					t.Fatal(errors.Unwrap(err))
				}
				lines := strings.Split(strings.TrimSpace(output.String()), "\n")
				if len(lines) != 6 || diagnostics.Len() != 0 {
					t.Fatalf("unexpected STDIO boundaries: %d frames, diagnostics=%q", len(lines), diagnostics.String())
				}
				var discovery struct {
					Result struct{ Tools []protocol.Tool }
				}
				if err := json.Unmarshal([]byte(lines[0]), &discovery); err != nil {
					t.Fatal(err)
				}
				var names []string
				for _, tool := range discovery.Result.Tools {
					names = append(names, tool.Name)
				}
				want := []string{"list_directory", "read_file", "get_path_info"}
				if tc.writable {
					want = append(want, "write_file", "create_directory", "delete_path", "copy_path", "move_path")
				}
				if !slices.Equal(names, want) {
					t.Fatalf("catalog = %v, want %v", names, want)
				}
				if !strings.Contains(lines[1], "ordinary sibling") || !strings.Contains(lines[2], "access_denied") ||
					!strings.Contains(lines[4], `"code":-32602`) || !strings.Contains(lines[5], `"code":-32602`) {
					t.Fatal("unexpected ordinary, configuration or future-tool call result")
				}
				for _, line := range lines {
					assertNoHostPathLeak(t, line)
				}
				created, err := os.ReadFile(filepath.Join(root, "created.txt"))
				if tc.writable {
					if err != nil || string(created) != "new" || !strings.Contains(lines[3], `"written":true`) {
						t.Fatalf("write failed: %s, %v", lines[3], err)
					}
				} else if !errors.Is(err, os.ErrNotExist) || !strings.Contains(lines[3], `"code":-32602`) {
					t.Fatalf("read view accepted a direct mutation: %s, %v", lines[3], err)
				}
			})
		}
	}
}

func TestStartupRejectsUnmeasuredExplicitGrants(t *testing.T) {
	for _, tc := range []struct{ grants, ceiling string }{
		{"[]", ""},
		{"['files.write']", ""},
		{"['files.read', 'search.run']", ""},
		{"['files.read', 'processes.control']", ""},
		{"['files.read', 'processes.control']", "max_risk = 'high'\n"},
		{"['files.read', 'files.write', 'processes.control']", "max_risk = 'high'\n"},
	} {
		path := writeConfig(t, configDocument(t, t.TempDir(), "[profiles]\nactive = 'private'\n"+tc.ceiling+"[profiles.private]\ngrants = "+tc.grants+"\n"))
		var stdout, stderr bytes.Buffer
		deps := defaultBootstrapDependencies(path)
		exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(ctx context.Context, _ string) error {
			return runWithIO(ctx, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), &stdout, io.Discard, deps)
		})
		if exitCode != 3 || stdout.Len() != 0 || stderr.String() != "flashgate-mcp: startup failed ("+string(config.CategoryInvalidProfile)+")\n" {
			t.Fatalf("unmeasured profile produced exit=%d, stdout=%q, stderr=%q", exitCode, stdout.String(), stderr.String())
		}
	}
}

func TestProfileWireAdmission(t *testing.T) {
	for _, readOnly := range []bool{true, false} {
		registry, _ := effectiveFixtureCatalog(t, readOnly, wireFileLimit, protocol.ProtocolVersion)
		if len(registry.List()) == 0 {
			t.Fatal("admission published an empty fixture catalog")
		}
	}
}

func TestProfileWireRejectsMissingAndOverBudgetBindings(t *testing.T) {
	const name = "read"
	const path = "production/" + protocol.ProtocolVersion + "/" + name + "/stdio-lf"
	_, view := effectiveFixtureCatalog(t, true, wireFileLimit, protocol.ProtocolVersion)
	fixture, err := profileWireFixtures.ReadFile("testdata/mcp/" + protocol.ProtocolVersion + "-" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		mutate     func(*activationBudgets, *wireSnapshot)
	}{
		{"missing executable path", "missing profile wire path", func(b *activationBudgets, _ *wireSnapshot) { delete(b.Paths, path) }},
		{"missing entry case", "missing profile wire case", func(b *activationBudgets, _ *wireSnapshot) { delete(b.Cases, "initialize///entry") }},
		{"missing call fixture", "missing profile wire case", func(_ *activationBudgets, s *wireSnapshot) { delete(s.Calls["read_file"], "success") }},
		{"stale revision", "invalid profile wire binding", func(_ *activationBudgets, s *wireSnapshot) { s.Revision = "unknown" }},
		{"missing operation inventory", "operation fixture inventory", func(_ *activationBudgets, s *wireSnapshot) { delete(s.OperationCases, "read_file") }},
		{"stale operation variant", "invalid or duplicate operation fixture", func(_ *activationBudgets, s *wireSnapshot) {
			s.OperationCases["read_file"][0].Operation = "filesystem.read/stale"
		}},
		{"wrong operation result", "invalid or duplicate operation fixture", func(_ *activationBudgets, s *wireSnapshot) {
			s.OperationCases["read_file"][0].Variant = "wrong"
		}},
		{"catalog oversize", "over-budget profile wire case", func(b *activationBudgets, _ *wireSnapshot) {
			class := b.Cases["tools/list///"+name].Class
			budget := b.Classes[class]
			budget.MaxResult = 1
			b.Classes[class] = budget
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var budgets activationBudgets
			var snapshot wireSnapshot
			if err := json.Unmarshal([]byte(wireBudgetManifest), &budgets); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(fixture, &snapshot); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&budgets, &snapshot)
			budgetBytes, _ := json.Marshal(budgets)
			fixtureBytes, _ := json.Marshal(snapshot)
			if err := checkProfileWire(view, protocol.ProtocolVersion, name, fixtureBytes, budgetBytes); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}
