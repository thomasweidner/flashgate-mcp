package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestFileTypesOnMCPWire(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{"allowed.txt": "allowed-marker", "denied.exe": "denied-marker"} {
		testconfig.Write(t, filepath.Join(root, name), []byte(data))
	}
	source := filepath.Join(t.TempDir(), "config.toml")
	testconfig.Write(t, source, []byte(configDocument(t, root, "roots.work.access=['read','write']\nroots.work.allowed_extensions=['.txt']\n"+profileSettings(false))))
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
	if err := admitProfile(registry, cfg.Profile().Policy()); err != nil {
		t.Fatal(err)
	}
	for _, params := range []string{
		`{"name":"read_file","arguments":{"root_id":"work","path":"denied.exe"}}`,
		`{"name":"get_path_info","arguments":{"root_id":"work","path":"denied.exe"}}`,
		`{"name":"write_file","arguments":{"root_id":"work","path":"denied.exe","content":"changed","overwrite":true}}`,
		`{"name":"delete_path","arguments":{"root_id":"work","path":"denied.exe"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"denied.exe","target":"allowed.txt","overwrite":true}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"allowed.txt","target":"denied.exe","overwrite":true}}`,
		`{"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"allowed.txt","target":"denied.exe","overwrite":true}}`,
	} {
		response, raw := runCallToolWireRequest(t, registry, params)
		if response.Error != nil || strings.Contains(raw, "denied-marker") || strings.Contains(raw, root) {
			t.Fatal(raw)
		}
		decoded, err := mcptest.DecodeCallToolResult(response.Result)
		if err != nil || !decoded.IsError || decoded.HasStructuredContent || decoded.TextContent.(map[string]any)["category"] != "access_denied" {
			t.Fatalf("unsafe denial: %s (%v)", raw, err)
		}
	}
	for name, want := range map[string]string{"allowed.txt": "allowed-marker", "denied.exe": "denied-marker"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(b) != want {
			t.Fatal("wire denial caused effect", err)
		}
	}
	for _, params := range []string{
		`{"name":"read_file","arguments":{"root_id":"work","path":"allowed.txt"}}`,
		`{"name":"list_directory","arguments":{"root_id":"work","path":"."}}`,
		`{"name":"create_directory","arguments":{"root_id":"work","path":"directory.exe"}}`,
	} {
		response, raw := runCallToolWireRequest(t, registry, params)
		var result struct {
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(response.Result, &result); err != nil || response.Error != nil || result.IsError {
			t.Fatal(raw, err)
		}
		if strings.Contains(params, "list_directory") && strings.Contains(raw, "denied.exe") {
			t.Fatal("denied name leaked", raw)
		}
	}
}
