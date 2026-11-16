package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestConfigurationProtectionOnMCPWire(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "settings")
	tree := filepath.Join(root, "tree")
	for _, path := range []string{dir, tree} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	const marker = "synthetic-config-content-374"
	source := filepath.Join(dir, "config.toml")
	document := "# " + marker + "\n" + configDocument(t, root,
		"max_file_size = 512\nroots.settings.path = "+quotedConfigPath(t, dir)+"\n[security]\nfollow_symlinks = true\n")
	testconfig.Write(t, source, []byte(document))
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	deps := defaultBootstrapDependencies(source)
	cfg, err := deps.loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	rootRegistry, err := deps.newRoots(cfg)
	if err != nil {
		t.Fatal(err)
	}
	registry := requireToolRegistry(t, rootRegistry, cfg.Files().MaxFileSize(), toolCapabilities{filesystemWrite: true})
	output := filepath.Join(dir, "output.txt")
	ordinary := filepath.Join(dir, "ordinary.txt")
	testconfig.Write(t, ordinary, []byte("ordinary"))
	alias := filepath.Join(tree, "alias.toml")
	if err := os.Link(source, alias); err != nil {
		t.Fatal(err)
	}

	call := func(params string, denied bool) string {
		t.Helper()
		response, raw := runCallToolWireRequest(t, registry, params)
		var result struct {
			IsError bool `json:"isError"`
		}
		err := json.Unmarshal(response.Result, &result)
		if err != nil || response.Error != nil || result.IsError != denied ||
			strings.Contains(raw, marker) || strings.Contains(raw, root) {
			t.Fatalf("unexpected response to %s: %s (%v)", params, raw, err)
		}
		if denied {
			decoded, err := mcptest.DecodeCallToolResult(response.Result)
			if err != nil || decoded.HasStructuredContent || decoded.TextContent.(map[string]any)["category"] != "access_denied" {
				t.Fatalf("unsafe or unexpected denial: %s", raw)
			}
		}
		return raw
	}
	for _, params := range []string{
		`{"name":"read_file","arguments":{"root_id":"work","path":"settings/config.toml"}}`,
		`{"name":"read_file","arguments":{"root_id":"settings","path":"config.toml"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"settings","target_root_id":"settings","source":"config.toml","target":"output.txt"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"settings","target_root_id":"settings","source":"ordinary.txt","target":"config.toml","overwrite":true}}`,
		`{"name":"read_file","arguments":{"root_id":"work","path":"tree/alias.toml"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"tree/alias.toml","target":"settings/output.txt"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"tree/alias.toml","target":"settings/ordinary.txt","overwrite":true}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings/ordinary.txt","target":"tree/alias.toml","overwrite":true}}`,
		`{"name":"write_file","arguments":{"root_id":"work","path":"settings/config.toml","content":"changed","overwrite":true}}`,
		`{"name":"write_file","arguments":{"root_id":"work","path":"tree/alias.toml","content":"changed","overwrite":true}}`,
		`{"name":"delete_path","arguments":{"root_id":"work","path":"tree/alias.toml"}}`,
		`{"name":"delete_path","arguments":{"root_id":"work","path":"settings","recursive":true}}`,
		`{"name":"delete_path","arguments":{"root_id":"work","path":"tree","recursive":true}}`,
		`{"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings","target":"moved"}}`,
		`{"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"tree","target":"moved"}}`,
		`{"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings/ordinary.txt","target":"settings/config.toml","overwrite":true}}`,
		`{"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings/config.toml","target":"moved"}}`,
	} {
		call(params, true)
		contents, err := os.ReadFile(source)
		current, statErr := os.Stat(source)
		unchanged, ordinaryErr := os.ReadFile(ordinary)
		if err != nil || statErr != nil || !os.SameFile(info, current) || string(contents) != document ||
			ordinaryErr != nil || string(unchanged) != "ordinary" {
			t.Fatal("denial changed the source or ordinary fixture")
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatalf("denied copy created output: %v", err)
		}
	}
	if runtime.GOOS == "windows" {
		call(`{"name":"read_file","arguments":{"root_id":"work","path":"SETTINGS/CONFIG.TOML"}}`, true)
		call(`{"name":"write_file","arguments":{"root_id":"work","path":"SETTINGS/CONFIG.TOML","content":"changed","overwrite":true}}`, true)
	}
	link := filepath.Join(root, "symbolic.toml")
	if err := os.Symlink(source, link); err == nil {
		call(`{"name":"read_file","arguments":{"root_id":"work","path":"symbolic.toml"}}`, true)
		call(`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"symbolic.toml","target":"settings/output.txt"}}`, true)
	} else if runtime.GOOS == "linux" {
		t.Fatal(err)
	} else {
		t.Logf("native symbolic link unavailable: %v", err)
	}
	for _, params := range []string{
		`{"name":"get_path_info","arguments":{"root_id":"work","path":"settings/config.toml"}}`,
		`{"name":"list_directory","arguments":{"root_id":"work","path":"settings"}}`,
		`{"name":"read_file","arguments":{"root_id":"work","path":"settings/ordinary.txt"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings/ordinary.txt","target":"settings/output.txt"}}`,
		`{"name":"copy_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings/ordinary.txt","target":"settings/output.txt","overwrite":true}}`,
		`{"name":"write_file","arguments":{"root_id":"work","path":"settings/output.txt","content":"updated","overwrite":true}}`,
		`{"name":"create_directory","arguments":{"root_id":"work","path":"settings/sibling/child"}}`,
		`{"name":"move_path","arguments":{"source_root_id":"work","target_root_id":"work","source":"settings/output.txt","target":"settings/sibling/child/output.txt"}}`,
		`{"name":"delete_path","arguments":{"root_id":"work","path":"settings/sibling","recursive":true}}`,
	} {
		call(params, false)
	}

	inPlace := "# " + marker + "\n" + configDocument(t, root, "max_file_size = 384\n")
	if err := os.WriteFile(source, []byte(inPlace), 0o600); err != nil {
		t.Fatal(err)
	}
	if next, err := deps.loadConfig(); err != nil || next.Files().MaxFileSize() != 384 ||
		cfg.Files().MaxFileSize() != 512 {
		t.Fatalf("in-place external edit changed the active snapshot: %v", err)
	}
	if err := os.Rename(source, filepath.Join(dir, "moved.toml")); err != nil {
		t.Fatal(err)
	}
	call(`{"name":"create_directory","arguments":{"root_id":"work","path":"settings/config.toml/child"}}`, true)
	call(`{"name":"write_file","arguments":{"root_id":"work","path":"settings/config.toml","content":"new"}}`, true)
	replacement := "# " + marker + "\n" + configDocument(t, root, "max_file_size = 256\n")
	testconfig.Write(t, source, []byte(replacement))
	if err := os.Link(source, filepath.Join(root, "replacement.toml")); err != nil {
		t.Fatal(err)
	}
	call(`{"name":"read_file","arguments":{"root_id":"work","path":"settings/config.toml"}}`, true)
	call(`{"name":"read_file","arguments":{"root_id":"work","path":"replacement.toml"}}`, true)
	call(`{"name":"read_file","arguments":{"root_id":"work","path":"settings/moved.toml"}}`, true)
	call(`{"name":"read_file","arguments":{"root_id":"work","path":"settings/ordinary.txt"}}`, false)
	testconfig.Write(t, filepath.Join(dir, "large.txt"), []byte(strings.Repeat("Q", 400)))
	if result := call(`{"name":"read_file","arguments":{"root_id":"work","path":"settings/large.txt"}}`, false); !strings.Contains(result, strings.Repeat("Q", 400)) {
		t.Fatal("ordinary MCP read did not use the active snapshot limit")
	}
	restart, err := deps.loadConfig()
	if err != nil || restart.Files().MaxFileSize() != 256 || cfg.Files().MaxFileSize() != 512 {
		t.Fatalf("running and restarted snapshots were not isolated: %v", err)
	}
}
