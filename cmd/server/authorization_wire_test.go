package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestProductionToolAuthorization(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	rootPath := quotedConfigPath(t, root)
	document := configDocument(t, root,
		"work.access = ['files.read', 'files.write']\n"+
			"reader.path = "+rootPath+"\n"+
			"writer.path = "+rootPath+"\nwriter.access = ['files.write']\n"+profileSettings(false))
	testconfig.Write(t, source, []byte(document))
	deps := defaultBootstrapDependencies(source)
	cfg, err := deps.loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	roots, err := deps.newRoots(cfg)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := createToolRegistry(roots, cfg.Files().MaxFileSize(), cfg.Profile())
	if err != nil {
		t.Fatal(err)
	}
	if err := admitProfile(registry); err != nil {
		t.Fatal(err)
	}
	if len(registry.List()) != 8 {
		t.Fatal("missing production registrations")
	}
	for _, tool := range registry.List() {
		if _, err := tool.Execute(t.Context(), nil); !errors.Is(err, execution.ErrMismatch) {
			t.Fatalf("%s exposed an executable metadata route: %v", tool.Name(), err)
		}
		if err := registry.Register(tool); !errors.Is(err, execution.ErrMismatch) {
			t.Fatalf("%s could be registered without request binding: %v", tool.Name(), err)
		}
	}

	testconfig.Write(t, filepath.Join(root, "source.txt"), []byte("source content"))
	testconfig.Write(t, filepath.Join(root, "move.txt"), []byte("move content"))
	for _, target := range []string{"write.txt", "copy-target.txt", "move-target.txt"} {
		testconfig.Write(t, filepath.Join(root, target), []byte("original"))
	}
	if err := os.Mkdir(filepath.Join(root, "remove"), 0o700); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, filepath.Join(root, "remove", "child.txt"), []byte("child"))

	assertFile := func(t *testing.T, path, want string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, err = %v, want %q", path, data, err, want)
		}
	}
	assertAbsent := func(t *testing.T, path string) {
		t.Helper()
		if _, err := os.Lstat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("%s is not absent: %v", path, err)
		}
	}
	for _, tc := range []struct {
		name        string
		arguments   string
		deniedRoots []string
		before      func(*testing.T)
		after       func(*testing.T)
	}{
		{"list_directory", `"root_id":"%s","path":"."`, []string{"writer"}, nil, nil},
		{"read_file", `"root_id":"%s","path":"source.txt"`, []string{"writer"}, nil, nil},
		{"get_path_info", `"root_id":"%s","path":"source.txt"`, []string{"writer"}, nil, nil},
		{"write_file", `"root_id":"%s","path":"write.txt","content":"updated","overwrite":true`, []string{"reader"},
			func(t *testing.T) { assertFile(t, "write.txt", "original") },
			func(t *testing.T) { assertFile(t, "write.txt", "updated") }},
		{"create_directory", `"root_id":"%s","path":"created"`, []string{"reader"},
			func(t *testing.T) { assertAbsent(t, "created") },
			func(t *testing.T) {
				if info, err := os.Stat(filepath.Join(root, "created")); err != nil || !info.IsDir() {
					t.Fatalf("directory was not created: %v", err)
				}
			}},
		{"delete_path", `"root_id":"%s","path":"remove","recursive":true`, []string{"reader"},
			func(t *testing.T) { assertFile(t, "remove/child.txt", "child") },
			func(t *testing.T) { assertAbsent(t, "remove") }},
		{"copy_path", `"source_root_id":"%[1]s","target_root_id":"%[1]s","source":"source.txt","target":"copy-target.txt","overwrite":true`, []string{"writer", "reader"},
			func(t *testing.T) { assertFile(t, "copy-target.txt", "original") },
			func(t *testing.T) { assertFile(t, "copy-target.txt", "source content") }},
		{"move_path", `"source_root_id":"%[1]s","target_root_id":"%[1]s","source":"move.txt","target":"move-target.txt","overwrite":true`, []string{"reader"},
			func(t *testing.T) {
				assertFile(t, "move.txt", "move content")
				assertFile(t, "move-target.txt", "original")
			},
			func(t *testing.T) {
				assertAbsent(t, "move.txt")
				assertFile(t, "move-target.txt", "move content")
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := func(rootID string, denied bool) {
				t.Helper()
				args := fmt.Sprintf(tc.arguments, rootID)
				params, err := json.Marshal(map[string]any{
					"name": tc.name, "arguments": json.RawMessage("{" + args + "}"),
				})
				if err != nil {
					t.Fatal(err)
				}
				response, raw := runCallToolWireRequest(t, registry, string(params))
				decode := mcptest.DecodeCallToolResult
				if !denied && (tc.name == "list_directory" || tc.name == "read_file") {
					decode = mcptest.DecodeSplitCallToolResult
				}
				result, err := decode(response.Result)
				if err != nil || response.Error != nil || result.IsError != denied {
					t.Fatalf("%s: %s (%v)", rootID, raw, err)
				}
				if denied {
					body, ok := result.TextContent.(map[string]any)
					if !ok || body["category"] != "access_denied" || result.HasStructuredContent {
						t.Fatalf("unsafe or unexpected denial: %s", raw)
					}
				}
			}
			for _, rootID := range tc.deniedRoots {
				call(rootID, true)
				if tc.before != nil {
					tc.before(t)
				}
			}
			call("work", false)
			if tc.after != nil {
				tc.after(t)
			}
		})
	}
}
