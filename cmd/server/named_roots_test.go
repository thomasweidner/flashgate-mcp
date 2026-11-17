package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestFilesystemNamedRootWire(t *testing.T) {
	work, archive := t.TempDir(), t.TempDir()
	for root, value := range map[string]string{work: "work seed", archive: "archive seed"} {
		testconfig.Write(t, filepath.Join(root, "seed.txt"), []byte(value))
	}
	source := writeConfig(t, configDocument(t, work,
		"roots.archive.path = "+quotedConfigPath(t, archive)+"\n"+
			"roots.archive.access = ['read', 'write']\n"+
			"roots.alias.path = "+quotedConfigPath(t, archive)+"\n"))
	cfg, err := config.Load(config.Source{Role: config.Direct, Path: source})
	if err != nil {
		t.Fatal(err)
	}
	filesystem, err := roots.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	registry := requireToolRegistry(t, filesystem, cfg.Files().MaxFileSize(), false)
	router := mcpRouter(registry, false)
	call := func(name, args string) string {
		t.Helper()
		request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` +
			name + `","arguments":` + args + `}}` + "\n"
		response := mcpWire(t, router, request, 1)[0]
		wire := string(response["result"])
		assertNoHostPathLeak(t, wire)
		return wire
	}
	for _, test := range []struct {
		name, args, want string
	}{
		{"list_directory", `{"root_id":"archive","path":"."}`, `"entries":[{"name":"seed.txt"`},
		{"read_file", `{"root_id":"archive","path":"seed.txt"}`, `"text":"archive seed"`},
		{"get_path_info", `{"root_id":"archive","path":"seed.txt"}`, `"exists":true`},
		{"create_directory", `{"root_id":"archive","path":"nested"}`, `"created":true`},
		{"write_file", `{"root_id":"archive","path":"nested/new.txt","content":"new"}`, `"written":true`},
		{"copy_path", `{"source_root_id":"archive","source":"seed.txt","target_root_id":"archive","target":"nested/copied.txt"}`, `"copied":true`},
		{"move_path", `{"source_root_id":"archive","source":"nested/copied.txt","target_root_id":"archive","target":"nested/moved.txt"}`, `"moved":true`},
		{"delete_path", `{"root_id":"archive","path":"nested/moved.txt"}`, `"deleted":true`},
	} {
		if wire := call(test.name, test.args); !strings.Contains(wire, test.want) || strings.Contains(wire, `"isError":true`) {
			t.Fatalf("%s: unexpected result %s", test.name, wire)
		}
	}
	if data, err := os.ReadFile(filepath.Join(archive, "nested", "new.txt")); err != nil || string(data) != "new" {
		t.Fatalf("expected write in archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "nested")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected effect in work root: %v", err)
	}
	for _, test := range []struct{ name, args, category string }{
		{"write_file", `{"root_id":"work","path":"seed.txt","content":"replace","overwrite":true}`, "access_denied"},
		{"write_file", `{"root_id":"alias","path":"seed.txt","content":"replace","overwrite":true}`, "access_denied"},
		{"copy_path", `{"source_root_id":"alias","source":"seed.txt","target_root_id":"alias","target":"seed.txt","overwrite":true}`, "access_denied"},
		{"list_directory", `{"root_id":"archive"}`, "invalid_arguments"},
		{"list_directory", `{"root_id":"missing","path":"."}`, "invalid_arguments"},
		{"read_file", `{"path":"seed.txt"}`, "invalid_arguments"},
		{"read_file", `{"root_id":"","path":"seed.txt"}`, "invalid_arguments"},
		{"read_file", `{"root_id":"Archive","path":"seed.txt"}`, "invalid_arguments"},
		{"read_file", `{"root_id":"missing","path":"seed.txt"}`, "invalid_arguments"},
		{"read_file", `{"rootId":"archive","path":"seed.txt"}`, "invalid_arguments"},
		{"read_file", `{"ROOT_ID":"archive","path":"seed.txt"}`, "invalid_arguments"},
		{"read_file", `{"root_id":"work","root_id":"archive","path":"seed.txt"}`, "invalid_arguments"},
		{"get_path_info", `{"root_id":"missing","path":"missing.txt"}`, "invalid_arguments"},
		{"write_file", `{"root_id":"missing","path":"seed.txt","content":"replace","overwrite":true}`, "invalid_arguments"},
		{"create_directory", `{"root_id":"missing","path":"denied.txt"}`, "invalid_arguments"},
		{"delete_path", `{"root_id":"missing","path":"seed.txt"}`, "invalid_arguments"},
		{"copy_path", `{"source":"seed.txt","target_root_id":"archive","target":"denied.txt"}`, "invalid_arguments"},
		{"move_path", `{"source_root_id":"archive","source":"seed.txt","target":"denied.txt"}`, "invalid_arguments"},
		{"move_path", `{"source_root_id":"missing","source":"seed.txt","target_root_id":"archive","target":"denied.txt"}`, "invalid_arguments"},
		{"copy_path", `{"source_root_id":"archive","source":"seed.txt","target_root_id":"missing","target":"denied.txt"}`, "invalid_arguments"},
		{"copy_path", `{"source_root_id":"work","source":"seed.txt","target_root_id":"archive","target":"denied.txt"}`, "unsupported_operation"},
		{"move_path", `{"source_root_id":"archive","source":"seed.txt","target_root_id":"alias","target":"denied.txt"}`, "unsupported_operation"},
	} {
		if wire := call(test.name, test.args); !strings.Contains(wire, test.category) || !strings.Contains(wire, `"isError":true`) ||
			strings.Contains(wire, `"structuredContent"`) || strings.Contains(wire, "seed.txt") {
			t.Errorf("%s %s: unexpected denial %s", test.name, test.args, wire)
		}
	}
	for root, want := range map[string]string{work: "work seed", archive: "archive seed"} {
		if _, err := os.Stat(filepath.Join(root, "denied.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("denied operation affected the target: %v", err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "seed.txt")); err != nil || string(data) != want {
			t.Errorf("denied operation affected the source: %v", err)
		}
	}
	readOnly := requireToolRegistry(t, filesystem, cfg.Files().MaxFileSize(), true)
	for _, name := range []string{"write_file", "create_directory", "delete_path", "copy_path", "move_path"} {
		if _, ok := readOnly.Get(name); ok {
			t.Errorf("root selection enabled %s in read-only mode", name)
		}
	}
}
