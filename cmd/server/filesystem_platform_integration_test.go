package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestFilesystemToolsCallPlatformIntegration(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("BL-066 platform integration contract targets Windows and Linux")
	}

	root := t.TempDir()
	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	registry := createToolRegistry(filesystem, 1024*1024, toolCapabilities{filesystemWrite: true})

	call := func(name string, arguments map[string]any) map[string]any {
		t.Helper()
		params, err := json.Marshal(map[string]any{"name": name, "arguments": arguments})
		if err != nil {
			t.Fatal(err)
		}
		response, raw := runCallToolWireRequest(t, registry, string(params))
		if response.Error != nil {
			t.Fatalf("%s returned an error: %#v; response=%s", name, response.Error, raw)
		}
		decoded, err := mcptest.DecodeCallToolResult(response.Result)
		if err != nil {
			t.Fatalf("%s returned an invalid CallToolResult: %v; response=%s", name, err, raw)
		}
		if decoded.IsError || !decoded.HasStructuredContent {
			t.Fatalf("%s returned an unexpected result: %#v", name, decoded)
		}
		result, ok := decoded.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("%s returned non-object structured content: %#v", name, decoded.StructuredContent)
		}
		return result
	}

	directory := filepath.ToSlash(filepath.Join("platform ü", "nested directory"))
	if result := call("create_directory", map[string]any{"path": directory}); result["created"] != true {
		t.Fatalf("create_directory did not create the platform fixture: %#v", result)
	}

	source := filepath.ToSlash(filepath.Join(directory, "source ä.txt"))
	content := "Windows/Linux tools/call integration\n"
	if result := call("write_file", map[string]any{"path": source, "content": content}); result["written"] != true {
		t.Fatalf("write_file did not report a write: %#v", result)
	}
	if result := call("read_file", map[string]any{"path": source}); result["content"] != content {
		t.Fatalf("read_file content mismatch: %#v", result)
	}
	if result := call("get_path_info", map[string]any{"path": source}); result["exists"] != true || result["isDir"] != false {
		t.Fatalf("get_path_info metadata mismatch: %#v", result)
	}
	if result := call("list_directory", map[string]any{"path": directory}); len(result["entries"].([]any)) != 1 {
		t.Fatalf("list_directory entry mismatch: %#v", result)
	}

	copyPath := filepath.ToSlash(filepath.Join(directory, "copy ä.txt"))
	if result := call("copy_path", map[string]any{"source": source, "target": copyPath}); result["copied"] != true {
		t.Fatalf("copy_path did not report a copy: %#v", result)
	}
	movePath := filepath.ToSlash(filepath.Join(directory, "moved ä.txt"))
	if result := call("move_path", map[string]any{"source": copyPath, "target": movePath}); result["moved"] != true {
		t.Fatalf("move_path did not report a move: %#v", result)
	}
	if result := call("delete_path", map[string]any{"path": movePath}); result["deleted"] != true {
		t.Fatalf("delete_path did not report a deletion: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(movePath))); !os.IsNotExist(err) {
		t.Fatalf("delete_path left its target behind: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	for name, path := range map[string]string{
		"absolute":  outside,
		"traversal": filepath.ToSlash(filepath.Join("..", filepath.Base(outside))),
	} {
		t.Run("reject_"+name, func(t *testing.T) {
			params, err := json.Marshal(map[string]any{
				"name":      "read_file",
				"arguments": map[string]any{"path": path},
			})
			if err != nil {
				t.Fatal(err)
			}
			response, raw := runCallToolWireRequest(t, registry, string(params))
			if response.Error == nil || response.Error.Code != protocol.ErrInvalidParams {
				t.Fatalf("unsafe path was not rejected: %s", raw)
			}
			if strings.Contains(raw, root) || strings.Contains(raw, outside) {
				t.Fatalf("path rejection leaked a host path: %s", raw)
			}
		})
	}
}
