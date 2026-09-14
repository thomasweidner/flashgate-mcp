package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestWriteFileToolDefinition(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	if tool.Name() != "write_file" {
		t.Fatalf("expected write_file, got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Fatal("expected description")
	}

	if tool.InputSchema() == nil {
		t.Fatal("expected input schema")
	}

	definition := tool.Definition()

	if definition.Name != "write_file" {
		t.Fatalf("expected definition name write_file, got %q", definition.Name)
	}

	if definition.Description == "" {
		t.Fatal("expected definition description")
	}

	if definition.InputSchema == nil {
		t.Fatal("expected definition input schema")
	}
}

func TestWriteFileToolExecute(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"tmp-write-test.txt","content":"hello world","overwrite":false}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.writePath != "tmp-write-test.txt" {
		t.Fatalf("expected path tmp-write-test.txt, got %q", filesystem.writePath)
	}

	if string(filesystem.writeContent) != "hello world" {
		t.Fatalf("expected content hello world, got %q", string(filesystem.writeContent))
	}

	if filesystem.writeOverwrite {
		t.Fatal("expected overwrite=false")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Path    string `json:"path"`
		Size    int64  `json:"size"`
		Written bool   `json:"written"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if decoded.Path != "tmp-write-test.txt" {
		t.Fatalf("expected path tmp-write-test.txt, got %q", decoded.Path)
	}

	if decoded.Size != int64(len("hello world")) {
		t.Fatalf("expected size %d, got %d", len("hello world"), decoded.Size)
	}

	if !decoded.Written {
		t.Fatal("expected written=true")
	}
}

func TestWriteFileToolAllowsEmptyContent(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"empty.txt"}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.writePath != "empty.txt" {
		t.Fatalf("expected path empty.txt, got %q", filesystem.writePath)
	}

	if len(filesystem.writeContent) != 0 {
		t.Fatalf("expected empty content, got %q", string(filesystem.writeContent))
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Size int64 `json:"size"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if decoded.Size != 0 {
		t.Fatalf("expected size 0, got %d", decoded.Size)
	}
}

func TestWriteFileToolForwardsOverwrite(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"existing.txt","content":"new","overwrite":true}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if !filesystem.writeOverwrite {
		t.Fatal("expected overwrite=true")
	}
}

func TestWriteFileToolForwardsAtomic(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"existing.txt","content":"new","overwrite":true,"atomic":true}`),
	)
	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}
	if !filesystem.writeAtomic {
		t.Fatal("expected atomic=true")
	}
}

func TestWriteFileToolReturnsInvalidParamsForMalformedJSON(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}

func TestWriteFileToolReturnsInvalidParamsForMissingPath(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewWriteFileTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"content":"hello"}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}

func TestWriteFileToolReturnsInvalidParamsForFileExists(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.writeErr = fs.ErrFileExists

	tool := NewWriteFileTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"existing.txt","content":"hello","overwrite":false}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}

func TestWriteFileToolReturnsInvalidParamsForFilesystemError(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.writeErr = fs.ErrPathIsDirectory

	tool := NewWriteFileTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"docs","content":"hello","overwrite":false}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}
