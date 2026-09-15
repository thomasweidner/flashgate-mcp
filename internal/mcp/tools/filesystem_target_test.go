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
		{ID: roots.DefaultID, FileSystem: defaultFS, Access: roots.ReadWrite, Limits: roots.DefaultLimits(1024, 2048), FileTypes: roots.AllFileTypes()},
		{ID: "notes", FileSystem: notesFS, Access: roots.Read, Limits: roots.DefaultLimits(1024, 2048), FileTypes: roots.AllFileTypes()},
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
		{ID: "read-only", FileSystem: filesystem, Access: roots.Read, Limits: roots.DefaultLimits(1024, 2048), FileTypes: roots.AllFileTypes()},
		{ID: "write-only", FileSystem: filesystem, Access: roots.Write, Limits: roots.DefaultLimits(1024, 2048), FileTypes: roots.AllFileTypes()},
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

func TestReadFileEnforcesSelectedRootFileAndResultLimits(t *testing.T) {
	filesystem := newFakeFileSystem()
	registry, err := roots.New([]roots.Entry{
		{ID: "file-limited", FileSystem: filesystem, Access: roots.Read, Limits: roots.Limits{
			MaxFileBytes: 64, MaxResultBytes: 256, MaxScanBytes: 512, MaxTemporaryBytes: 512,
		}, FileTypes: roots.AllFileTypes()},
		{ID: "result-limited", FileSystem: filesystem, Access: roots.Read, Limits: roots.Limits{
			MaxFileBytes: 256, MaxResultBytes: 32, MaxScanBytes: 512, MaxTemporaryBytes: 512,
		}, FileTypes: roots.AllFileTypes()},
	})
	if err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool(filesystem, 1024)
	BindRootRegistry(tool, registry)
	for rootID, want := range map[string]int64{"file-limited": 64, "result-limited": 32} {
		_, rpcErr := tool.Execute(context.Background(), json.RawMessage(
			`{"rootId":"`+rootID+`","path":"data.txt","maxBytes":900}`,
		))
		if rpcErr != nil {
			t.Fatalf("%s: unexpected error: %#v", rootID, rpcErr)
		}
		if filesystem.readMaxBytes != want {
			t.Fatalf("%s: read maximum = %d, want %d", rootID, filesystem.readMaxBytes, want)
		}
	}
}

func TestFileContentToolsEnforceSelectedRootFileTypes(t *testing.T) {
	filesystem := newFakeFileSystem()
	registry, err := roots.New([]roots.Entry{{
		ID: "text", FileSystem: filesystem, Access: roots.ReadWrite,
		Limits:    roots.DefaultLimits(1024, 2048),
		FileTypes: roots.FileTypes{Extensions: []string{".md", ".txt"}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	readTool := NewReadFileTool(filesystem, 1024)
	BindRootRegistry(readTool, registry)
	if _, rpcErr := readTool.Execute(context.Background(), json.RawMessage(`{"rootId":"text","path":"docs\\README.TXT"}`)); rpcErr != nil {
		t.Fatalf("portable case-insensitive read match failed: %#v", rpcErr)
	}
	if result, rpcErr := readTool.Execute(context.Background(), json.RawMessage(`{"rootId":"text","path":"archive.exe"}`)); result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("disallowed read = (%#v, %#v)", result, rpcErr)
	}

	writeTool := NewWriteFileTool(filesystem)
	BindRootRegistry(writeTool, registry)
	if result, rpcErr := writeTool.Execute(context.Background(), json.RawMessage(`{"rootId":"text","path":"archive.exe","content":"blocked"}`)); result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("disallowed write = (%#v, %#v)", result, rpcErr)
	}
	if filesystem.writePath != "" {
		t.Fatalf("disallowed type reached filesystem with path %q", filesystem.writePath)
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
