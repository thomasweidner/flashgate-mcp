package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

func TestFilesystemToolSelectsConfiguredRootByOpaqueID(t *testing.T) {
	defaultFS := newFakeFileSystem()
	notesFS := newFakeFileSystem()
	notesFS.readContent = []byte("selected")
	registry, err := roots.New([]roots.Entry{
		{ID: roots.DefaultID, FileSystem: defaultFS, Access: roots.ReadWrite},
		{ID: "notes", FileSystem: notesFS, Access: roots.Read},
	})
	if err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool(defaultFS, 1024)
	BindRootRegistry(tool, registry)
	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(
		`{"rootId":"notes","path":"daily.txt"}`,
	))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
	if notesFS.readPath != "daily.txt" || defaultFS.readPath != "" {
		t.Fatalf("wrong root selected: default=%q notes=%q", defaultFS.readPath, notesFS.readPath)
	}
	if result != (readFileResult{Content: "selected", Size: 8}) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestFilesystemToolsEnforcePerRootReadWritePolicy(t *testing.T) {
	filesystem := newFakeFileSystem()
	registry, err := roots.New([]roots.Entry{
		{ID: "read-only", FileSystem: filesystem, Access: roots.Read},
		{ID: "write-only", FileSystem: filesystem, Access: roots.Write},
	})
	if err != nil {
		t.Fatal(err)
	}

	readTool := NewReadFileTool(filesystem, 1024)
	BindRootRegistry(readTool, registry)
	if result, rpcErr := readTool.Execute(context.Background(), json.RawMessage(
		`{"rootId":"write-only","path":"secret.txt"}`,
	)); result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("read against write-only root = (%#v, %#v)", result, rpcErr)
	}

	writeTool := NewWriteFileTool(filesystem)
	BindRootRegistry(writeTool, registry)
	if result, rpcErr := writeTool.Execute(context.Background(), json.RawMessage(
		`{"rootId":"read-only","path":"blocked.txt","content":"blocked"}`,
	)); result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("write against read-only root = (%#v, %#v)", result, rpcErr)
	}
	if filesystem.writePath != "" {
		t.Fatalf("denied write reached filesystem with path %q", filesystem.writePath)
	}
}

func TestFilesystemToolRejectsUnknownRootWithoutFilesystemAccess(t *testing.T) {
	filesystem := newFakeFileSystem()
	tool := NewReadFileTool(filesystem, 1024)

	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(
		`{"rootId":"unknown","path":"secret.txt"}`,
	))
	if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected generic invalid params, result=%#v error=%#v", result, rpcErr)
	}
	if filesystem.readPath != "" {
		t.Fatalf("unknown root reached filesystem with path %q", filesystem.readPath)
	}
}
