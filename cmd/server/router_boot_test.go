package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestCreateRouterRegistersInitialize(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, capabilitiesFromReadOnly(false))
	mcpRouter := createRouter("test-server", "test-version", registry)

	params := json.RawMessage(`{
        "protocolVersion": "2025-11-25",
        "capabilities": {},
        "clientInfo": {
            "name": "test-client",
            "version": "test-version"
        }
    }`)

	result, protocolErr := mcpRouter.Dispatch(
		"initialize",
		handlers.Context{Context: context.Background()},
		params,
	)

	if protocolErr != nil {
		t.Fatalf("expected initialize to be registered, got protocol error: %+v", protocolErr)
	}

	if result == nil {
		t.Fatal("expected initialize result")
	}
}

func TestCreateRouterRegistersToolsList(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, capabilitiesFromReadOnly(false))
	mcpRouter := createRouter("test-server", "test-version", registry)

	result, protocolErr := mcpRouter.Dispatch(
		"tools/list",
		handlers.Context{Context: context.Background()},
		json.RawMessage(`{}`),
	)

	if protocolErr != nil {
		t.Fatalf("expected tools/list to be registered, got protocol error: %+v", protocolErr)
	}

	if result == nil {
		t.Fatal("expected tools/list result")
	}
}

func TestCreateRouterRegistersToolsCall(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, capabilitiesFromReadOnly(false))
	mcpRouter := createRouter("test-server", "test-version", registry)

	_, protocolErr := mcpRouter.Dispatch(
		"tools/call",
		handlers.Context{Context: context.Background()},
		json.RawMessage(`{}`),
	)

	if protocolErr == nil {
		t.Fatal("expected protocol error for invalid tools/call params")
	}

	if protocolErr.Code == protocol.ErrMethodNotFound {
		t.Fatalf("expected tools/call to be registered, got method-not-found error: %+v", protocolErr)
	}

	if protocolErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected invalid params from registered tools/call handler, got: %+v", protocolErr)
	}
}

func TestCreateRouterRejectsWriteToolCallWhenReadOnly(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, capabilitiesFromReadOnly(true))
	mcpRouter := createRouter("test-server", "test-version", registry)

	_, protocolErr := mcpRouter.Dispatch(
		"tools/call",
		handlers.Context{Context: context.Background()},
		json.RawMessage(`{"name":"write_file","arguments":{"path":"out.txt","content":"blocked"}}`),
	)

	if protocolErr == nil {
		t.Fatal("expected protocol error for disabled write tool")
	}

	if protocolErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected invalid params for disabled write tool, got: %+v", protocolErr)
	}
}

func TestCreateRouterRejectsUnknownMethod(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, capabilitiesFromReadOnly(false))
	mcpRouter := createRouter("test-server", "test-version", registry)

	_, protocolErr := mcpRouter.Dispatch(
		"unknown/method",
		handlers.Context{Context: context.Background()},
		json.RawMessage(`{}`),
	)

	if protocolErr == nil {
		t.Fatal("expected protocol error")
	}

	if protocolErr.Code != protocol.ErrMethodNotFound {
		t.Fatalf("expected method not found, got: %+v", protocolErr)
	}
}

func TestCreateRouterCallsGetPathInfoForMissingPath(t *testing.T) {
	filesystem, err := fs.NewLocalFileSystem(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	router := createRouter("test-server", "test-version", createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(false)))

	result, protocolErr := router.Dispatch("tools/call", handlers.Context{Context: context.Background()}, json.RawMessage(`{"name":"get_path_info","arguments":{"path":"missing.txt"}}`))
	if protocolErr != nil {
		t.Fatalf("unexpected error: %#v", protocolErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mcptest.DecodeCallToolResult(encoded)
	if err != nil {
		t.Fatalf("unexpected missing result: %v; JSON=%s", err, encoded)
	}
	missing, ok := decoded.StructuredContent.(map[string]any)
	if !ok || missing["path"] != "missing.txt" || missing["exists"] != false || len(missing) != 2 || decoded.IsError {
		t.Fatalf("unexpected missing result: %#v", decoded)
	}
}

func TestCreateRouterUsesMovePathForRename(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	router := createRouter("test-server", "test-version", createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(false)))

	_, protocolErr := router.Dispatch("tools/call", handlers.Context{Context: context.Background()}, json.RawMessage(`{"name":"move_path","arguments":{"source":"old.txt","target":"new.txt"}}`))
	if protocolErr != nil {
		t.Fatalf("unexpected error: %#v", protocolErr)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); err != nil {
		t.Fatalf("expected renamed file: %v", err)
	}
}

func TestReadOnlyRouterPositiveAndSecurityContract(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "unicode-ä"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"root.txt":                "root content",
		"sub directory/space.txt": "space content",
		"unicode-ä/ü.txt":         "unicode content",
	}
	for relative, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	mcpRouter := createRouter(
		"test-server",
		"test-version",
		createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(true)),
	)

	positiveCalls := []string{
		`{"name":"list_directory","arguments":{}}`,
		`{"name":"list_directory","arguments":{"path":"sub directory"}}`,
		`{"name":"read_file","arguments":{"path":"root.txt"}}`,
		`{"name":"read_file","arguments":{"path":"sub directory/space.txt"}}`,
		`{"name":"read_file","arguments":{"path":"unicode-ä/ü.txt"}}`,
		`{"name":"get_path_info","arguments":{"path":"root.txt"}}`,
		`{"name":"get_path_info","arguments":{"path":"sub directory"}}`,
		`{"name":"get_path_info","arguments":{"path":"missing.txt"}}`,
	}
	for _, raw := range positiveCalls {
		result, protocolErr := mcpRouter.Dispatch(
			"tools/call",
			handlers.Context{Context: context.Background()},
			json.RawMessage(raw),
		)
		if protocolErr != nil {
			t.Fatalf("call %s failed: %#v", raw, protocolErr)
		}
		if result == nil {
			t.Fatalf("call %s returned nil", raw)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := mcptest.DecodeCallToolResult(encoded); err != nil {
			t.Fatalf("call %s returned invalid CallToolResult: %v; JSON=%s", raw, err, encoded)
		}
	}

	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideArguments, err := json.Marshal(map[string]any{
		"name":      "read_file",
		"arguments": map[string]any{"path": outsidePath},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"name":"read_file","arguments":{"path":"../outside.txt"}}`),
		outsideArguments,
	} {
		_, protocolErr := mcpRouter.Dispatch(
			"tools/call",
			handlers.Context{Context: context.Background()},
			raw,
		)
		if protocolErr == nil || protocolErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected generic invalid params for outside path %s, got %#v", raw, protocolErr)
		}
		if protocolErr != nil && strings.Contains(protocolErr.Message, root) {
			t.Fatalf("protocol error leaked root: %q", protocolErr.Message)
		}
	}

	for _, blockedName := range []string{
		"write_file", "create_directory", "delete_path", "copy_path", "move_path",
		"list_files", "stat_path", "exists_path", "mkdir", "rename_path",
	} {
		raw, err := json.Marshal(map[string]any{"name": blockedName, "arguments": map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		_, protocolErr := mcpRouter.Dispatch(
			"tools/call",
			handlers.Context{Context: context.Background()},
			raw,
		)
		if protocolErr == nil || protocolErr.Code != protocol.ErrInvalidParams || protocolErr.Message != "invalid params" {
			t.Fatalf("expected generic invalid params for %q, got %#v", blockedName, protocolErr)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "blocked.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no read-only artifact, got %v", err)
	}
}
