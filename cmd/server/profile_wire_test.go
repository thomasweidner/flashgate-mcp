package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestEffectiveProfilesOnSTDIO(t *testing.T) {
	for _, tc := range []struct {
		name, settings string
		writable       bool
	}{
		{"default", "[profiles.definitions.future]\ngrants = ['search.run', 'system.read']\n", false},
		{"explicit read ceiling", "[profiles]\nmax_risk = 'read'\n", false},
		{"builtin write", profileSettings(false), true},
		{"custom write", "[profiles]\nactive = 'dev'\n[profiles.definitions.dev]\ngrants = ['files.write', 'files.read']\n", true},
		{"composed write", "[profiles]\nactive = 'read'\ninclude = ['changes']\n[profiles.definitions.changes]\ngrants = ['files.write']\n", true},
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
				settings := "roots.work.access = ['read', 'write']\n" + tc.settings
				if multiple {
					settings = "roots.second.path = " + quotedConfigPath(t, t.TempDir()) + "\n" + settings
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
		path := writeConfig(t, configDocument(t, t.TempDir(), "[profiles]\nactive = 'private'\n"+tc.ceiling+"[profiles.definitions.private]\ngrants = "+tc.grants+"\n"))
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
		profile := testProfile(t, readOnly)
		registry := requireToolRegistry(t, noopFileSystem{}, 1024, readOnly)
		if err := admitProfile(registry, profile.Policy()); err != nil {
			t.Fatalf("profile %s: %v", profile.ID(), err)
		}
	}
}

func TestProfileWireRejectsMissingAndOverBudgetBindings(t *testing.T) {
	const name = "read"
	const path = "production/" + protocol.ProtocolVersion + "/" + name + "/stdio-lf"
	registry := requireToolRegistry(t, noopFileSystem{}, 1024, true)
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
		{"missing call fixture", "incomplete profile result fixtures", func(_ *activationBudgets, s *wireSnapshot) { delete(s.Calls["read_file"], "success") }},
		{"stale revision", "invalid profile wire binding", func(_ *activationBudgets, s *wireSnapshot) { s.Revision = "unknown" }},
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
			if err := checkProfileWire(registry, protocol.ProtocolVersion, name, fixtureBytes, budgetBytes); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}
