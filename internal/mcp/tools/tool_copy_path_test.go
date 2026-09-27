package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestCopyPathToolDefinition(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewCopyPathTool(filesystem)

	if tool.Name() != "copy_path" {
		t.Fatalf("expected copy_path, got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Fatal("expected description")
	}

	if tool.InputSchema() == nil {
		t.Fatal("expected input schema")
	}

	definition := tool.Definition()

	if definition.Name != "copy_path" {
		t.Fatalf("expected definition name copy_path, got %q", definition.Name)
	}

	if definition.Description == "" {
		t.Fatal("expected definition description")
	}

	if definition.InputSchema == nil {
		t.Fatal("expected definition input schema")
	}
}

func TestCopyPathToolExecute(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewCopyPathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"tmp-copy-source.txt","target":"tmp-copy-target.txt"}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.copySource != "tmp-copy-source.txt" {
		t.Fatalf("expected source tmp-copy-source.txt, got %q", filesystem.copySource)
	}

	if filesystem.copyTarget != "tmp-copy-target.txt" {
		t.Fatalf("expected target tmp-copy-target.txt, got %q", filesystem.copyTarget)
	}

	if filesystem.copyOverwrite {
		t.Fatal("expected overwrite=false")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Copied bool   `json:"copied"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if decoded.Source != "tmp-copy-source.txt" {
		t.Fatalf("expected source tmp-copy-source.txt, got %q", decoded.Source)
	}

	if decoded.Target != "tmp-copy-target.txt" {
		t.Fatalf("expected target tmp-copy-target.txt, got %q", decoded.Target)
	}

	if !decoded.Copied {
		t.Fatal("expected copied=true")
	}
}

func TestCopyPathToolForwardsOverwrite(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewCopyPathTool(filesystem)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"source.txt","target":"target.txt","overwrite":true}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if !filesystem.copyOverwrite {
		t.Fatal("expected overwrite=true")
	}
}

func TestCopyPathToolReturnsInvalidParamsForMalformedJSON(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewCopyPathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":`),
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

func TestCopyPathToolReturnsInvalidParamsForMissingSource(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewCopyPathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"target":"target.txt"}`),
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

func TestCopyPathToolReturnsInvalidParamsForMissingTarget(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewCopyPathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"source.txt"}`),
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

func TestCopyPathToolReturnsInvalidParamsForTargetExists(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.copyErr = fs.ErrFileExists

	tool := NewCopyPathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"source.txt","target":"target.txt","overwrite":false}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "already_exists" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}

func TestCopyPathToolReturnsInvalidParamsForFilesystemError(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.copyErr = fs.ErrPathIsDirectory

	tool := NewCopyPathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"invalid","target":"target.txt"}`),
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
