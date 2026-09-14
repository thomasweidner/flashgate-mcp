package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type callToolWireResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *protocol.Error `json:"error,omitempty"`
}

func TestFilesystemCallToolWireSuccesses(t *testing.T) {
	root := t.TempDir()
	readContent := "line 1\nUnicode ÄÖÜ and folder\\relative\\file.txt"
	if err := os.WriteFile(filepath.Join(root, "read file.txt"), []byte(readContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("move"), 0o600); err != nil {
		t.Fatal(err)
	}
	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	registry := createToolRegistry(filesystem, 1024*1024, toolCapabilities{filesystemWrite: true})

	tests := []struct {
		name                  string
		params                string
		expectedResponseBytes int
		expectedResultBytes   int
		assertions            func(*testing.T, map[string]any)
	}{
		{
			name:                  "search_paths",
			params:                `{"name":"search_paths","arguments":{}}`,
			expectedResponseBytes: 275,
			expectedResultBytes:   240,
			assertions: func(t *testing.T, value map[string]any) {
				if _, ok := value["paths"].([]any); !ok {
					t.Fatalf("expected paths array, got %#v", value)
				}
			},
		},
		{
			name:                  "list_directory",
			params:                `{"name":"list_directory","arguments":{}}`,
			expectedResponseBytes: 321,
			expectedResultBytes:   286,
			assertions: func(t *testing.T, value map[string]any) {
				if _, ok := value["entries"].([]any); !ok {
					t.Fatalf("expected entries array, got %#v", value)
				}
			},
		},
		{
			name:                  "read_file content collision",
			params:                `{"name":"read_file","arguments":{"path":"read file.txt"}}`,
			expectedResponseBytes: 260,
			expectedResultBytes:   225,
			assertions: func(t *testing.T, value map[string]any) {
				if value["content"] != readContent || value["size"] != json.Number("50") {
					t.Fatalf("unexpected read result: %#v", value)
				}
			},
		},
		{
			name:                  "get_path_info existing",
			params:                `{"name":"get_path_info","arguments":{"path":"read file.txt"}}`,
			expectedResponseBytes: 279,
			expectedResultBytes:   244,
			assertions: func(t *testing.T, value map[string]any) {
				if value["exists"] != true || value["name"] != "read file.txt" {
					t.Fatalf("unexpected existing result: %#v", value)
				}
			},
		},
		{
			name:                  "get_path_info missing",
			params:                `{"name":"get_path_info","arguments":{"path":"does-not-exist.txt"}}`,
			expectedResponseBytes: 189,
			expectedResultBytes:   154,
			assertions: func(t *testing.T, value map[string]any) {
				if value["path"] != "does-not-exist.txt" || value["exists"] != false || len(value) != 2 {
					t.Fatalf("unexpected missing result: %#v", value)
				}
			},
		},
		{
			name:                  "move_path default profile",
			params:                `{"name":"move_path","arguments":{"source":"old.txt","target":"new.txt"}}`,
			expectedResponseBytes: 209,
			expectedResultBytes:   174,
			assertions: func(t *testing.T, value map[string]any) {
				if value["moved"] != true || value["source"] != "old.txt" || value["target"] != "new.txt" {
					t.Fatalf("unexpected move result: %#v", value)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, raw := runCallToolWireRequest(t, registry, tc.params)
			if got := len(raw) + 1; got != tc.expectedResponseBytes {
				t.Fatalf("response bytes=%d, want %d", got, tc.expectedResponseBytes)
			}
			if got := len(response.Result); got != tc.expectedResultBytes {
				t.Fatalf("result bytes=%d, want %d", got, tc.expectedResultBytes)
			}
			if response.Error != nil {
				t.Fatalf("unexpected error: %#v", response.Error)
			}
			decoded, err := mcptest.DecodeCallToolResult(response.Result)
			if err != nil {
				t.Fatalf("strict CallToolResult validation failed: %v; response=%s", err, raw)
			}
			if !decoded.HasStructuredContent || decoded.IsError {
				t.Fatalf("unexpected decoded result: %#v", decoded)
			}
			structured, ok := decoded.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("expected structured object, got %#v", decoded.StructuredContent)
			}
			tc.assertions(t, structured)
		})
	}

	if _, err := os.Stat(filepath.Join(root, "new.txt")); err != nil {
		t.Fatalf("expected move target: %v", err)
	}
}

func TestFilesystemCallToolWireErrorsRemainSafe(t *testing.T) {
	root := t.TempDir()
	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	defaultRegistry := createToolRegistry(filesystem, 1024, toolCapabilities{filesystemWrite: true})
	readOnlyRegistry := createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(true))

	tests := []struct {
		name                  string
		registry              *tools.Registry
		params                string
		message               string
		expectedResponseBytes int
	}{
		{"unknown tool", defaultRegistry, `{"name":"unknown_tool","arguments":{}}`, "invalid params", 76},
		{"gated write tool", readOnlyRegistry, `{"name":"write_file","arguments":{"path":"blocked.txt"}}`, "invalid params", 76},
		{"legacy tool", defaultRegistry, `{"name":"list_files","arguments":{}}`, "invalid params", 76},
		{"invalid arguments", defaultRegistry, `{"name":"get_path_info","arguments":{}}`, "invalid params", 76},
		{"PathGuard traversal", defaultRegistry, `{"name":"read_file","arguments":{"path":"..\\outside.txt"}}`, "filesystem error: invalid path", 92},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, raw := runCallToolWireRequest(t, tc.registry, tc.params)
			if got := len(raw) + 1; got != tc.expectedResponseBytes {
				t.Fatalf("response bytes=%d, want %d", got, tc.expectedResponseBytes)
			}
			if response.Error == nil || response.Error.Code != protocol.ErrInvalidParams || response.Error.Message != tc.message {
				t.Fatalf("unexpected error response: %s", raw)
			}
			if len(response.Result) != 0 {
				t.Fatalf("error response must not contain result: %s", raw)
			}
			if strings.Contains(raw, root) {
				t.Fatalf("response leaked root path: %s", raw)
			}
		})
	}
}

func runCallToolWireRequest(t *testing.T, registry *tools.Registry, params string) (callToolWireResponse, string) {
	t.Helper()
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + params + `}` + "\n"
	output := &bytes.Buffer{}
	server := mcpserver.New(strings.NewReader(request), output, createRouter("test-server", "test-version", registry))
	if err := server.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var response callToolWireResponse
	raw := strings.TrimSpace(output.String())
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("invalid response JSON: %v; response=%s", err, raw)
	}
	if response.JSONRPC != protocol.JSONRPCVersion || string(response.ID) != "1" {
		t.Fatalf("invalid response envelope: %s", raw)
	}
	return response, raw
}
