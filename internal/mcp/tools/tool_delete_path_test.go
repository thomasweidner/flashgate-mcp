package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestDeletePathToolDefinition(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewDeletePathTool(filesystem)

	if tool.Name() != "delete_path" {
		t.Fatalf("expected delete_path, got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Fatal("expected description")
	}

	if tool.InputSchema() == nil {
		t.Fatal("expected input schema")
	}

	definition := tool.Definition()

	if definition.Name != "delete_path" {
		t.Fatalf("expected definition name delete_path, got %q", definition.Name)
	}

	if definition.Description == "" {
		t.Fatal("expected definition description")
	}

	if definition.InputSchema == nil {
		t.Fatal("expected definition input schema")
	}
}

func TestDeletePathToolExecuteFile(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewDeletePathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"tmp-delete-test.txt"}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.deletePath != "tmp-delete-test.txt" {
		t.Fatalf("expected path tmp-delete-test.txt, got %q", filesystem.deletePath)
	}

	if filesystem.deleteRecursive {
		t.Fatal("expected recursive=false")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Path    string `json:"path"`
		Deleted bool   `json:"deleted"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if decoded.Path != "tmp-delete-test.txt" {
		t.Fatalf("expected path tmp-delete-test.txt, got %q", decoded.Path)
	}

	if !decoded.Deleted {
		t.Fatal("expected deleted=true")
	}
}

func TestDeletePathToolForwardsRecursive(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewDeletePathTool(filesystem)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"tmp-delete-dir","recursive":true}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.deletePath != "tmp-delete-dir" {
		t.Fatalf("expected path tmp-delete-dir, got %q", filesystem.deletePath)
	}

	if !filesystem.deleteRecursive {
		t.Fatal("expected recursive=true")
	}
}

func TestDeletePathToolReturnsInvalidParamsForMalformedJSON(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewDeletePathTool(filesystem)

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

	if errorCategory(rpcErr) != "invalid_arguments" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}

func TestDeletePathToolReturnsInvalidParamsForMissingPath(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewDeletePathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "invalid_arguments" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}

func TestDeletePathToolReturnsInvalidParamsForDirectoryNotEmpty(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.deleteErr = fs.ErrDirectoryNotEmpty

	tool := NewDeletePathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"non-empty-dir","recursive":false}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "unsupported_path_type" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}

func TestDeletePathToolReturnsInvalidParamsForFilesystemError(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.deleteErr = fs.ErrPathIsDirectory

	tool := NewDeletePathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"invalid"}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "unsupported_path_type" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}
