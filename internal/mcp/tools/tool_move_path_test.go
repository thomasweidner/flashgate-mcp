package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestMovePathToolDefinition(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewMovePathTool(filesystem)

	if tool.Name() != "move_path" {
		t.Fatalf("expected move_path, got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Fatal("expected description")
	}

	if tool.InputSchema() == nil {
		t.Fatal("expected input schema")
	}

	definition := tool.Definition()

	if definition.Name != "move_path" {
		t.Fatalf("expected definition name move_path, got %q", definition.Name)
	}

	if definition.Description == "" {
		t.Fatal("expected definition description")
	}

	if definition.InputSchema == nil {
		t.Fatal("expected definition input schema")
	}
}

func TestMovePathToolExecute(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewMovePathTool(filesystem)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"tmp-move-source.txt","target":"tmp-move-target.txt"}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.moveSource != "tmp-move-source.txt" {
		t.Fatalf("expected source tmp-move-source.txt, got %q", filesystem.moveSource)
	}

	if filesystem.moveTarget != "tmp-move-target.txt" {
		t.Fatalf("expected target tmp-move-target.txt, got %q", filesystem.moveTarget)
	}

	if filesystem.moveOverwrite {
		t.Fatal("expected overwrite=false")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Moved  bool   `json:"moved"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if decoded.Source != "tmp-move-source.txt" {
		t.Fatalf("expected source tmp-move-source.txt, got %q", decoded.Source)
	}

	if decoded.Target != "tmp-move-target.txt" {
		t.Fatalf("expected target tmp-move-target.txt, got %q", decoded.Target)
	}

	if !decoded.Moved {
		t.Fatal("expected moved=true")
	}
}

func TestMovePathToolForwardsOverwrite(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewMovePathTool(filesystem)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"source":"source.txt","target":"target.txt","overwrite":true}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if !filesystem.moveOverwrite {
		t.Fatal("expected overwrite=true")
	}
}

func TestMovePathToolReturnsInvalidParamsForMalformedJSON(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewMovePathTool(filesystem)

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

func TestMovePathToolReturnsInvalidParamsForMissingSource(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewMovePathTool(filesystem)

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

func TestMovePathToolReturnsInvalidParamsForMissingTarget(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewMovePathTool(filesystem)

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

func TestMovePathToolReturnsInvalidParamsForTargetExists(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.moveErr = fs.ErrFileExists

	tool := NewMovePathTool(filesystem)

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

func TestMovePathToolReturnsInvalidParamsForFilesystemError(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.moveErr = fs.ErrPathIsDirectory

	tool := NewMovePathTool(filesystem)

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
