package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestReadFileToolDefinition(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewReadFileTool(filesystem, 1024)

	if tool.Name() != "read_file" {
		t.Fatalf("expected read_file, got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Fatal("expected description")
	}

	if tool.InputSchema() == nil {
		t.Fatal("expected input schema")
	}

	definition := tool.Definition()

	if definition.Name != "read_file" {
		t.Fatalf("expected definition name read_file, got %q", definition.Name)
	}

	if definition.Description == "" {
		t.Fatal("expected definition description")
	}

	if definition.InputSchema == nil {
		t.Fatal("expected definition input schema")
	}
}

func TestReadFileToolExecute(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.readContent = []byte("hello world")

	tool := NewReadFileTool(filesystem, 1024)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"README.md","maxBytes":256}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.readPath != "README.md" {
		t.Fatalf("expected path README.md, got %q", filesystem.readPath)
	}

	if filesystem.readMaxBytes != 256 {
		t.Fatalf("expected maxBytes 256, got %d", filesystem.readMaxBytes)
	}

	if len(result.parts) != 1 || result.parts[0].kind != TextPayload || result.parts[0].text != "hello world" {
		t.Fatalf("unexpected text payload: %#v", result.parts)
	}

	if result.object != nil {
		t.Fatalf("unexpected caller-supplied metadata: %#v", result.object)
	}
}

func TestReadFileToolUsesDefaultMaxBytes(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.readContent = []byte("abc")

	tool := NewReadFileTool(filesystem, 4096)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"README.md"}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.readMaxBytes != 4096 {
		t.Fatalf("expected default maxBytes 4096, got %d", filesystem.readMaxBytes)
	}
}

func TestReadFileToolCapsClientMaxBytesAtServerLimit(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.readContent = []byte("abc")

	tool := NewReadFileTool(filesystem, 4096)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"README.md","maxBytes":8192}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if filesystem.readMaxBytes != 4096 {
		t.Fatalf("expected maxBytes capped at 4096, got %d", filesystem.readMaxBytes)
	}
}

func TestReadFileToolAcceptsOneByteLimit(t *testing.T) {
	filesystem := newFakeFileSystem()
	_, rpcErr := NewReadFileTool(filesystem, 4096).Execute(context.Background(), json.RawMessage(`{"path":"README.md","maxBytes":1}`))
	if rpcErr != nil || filesystem.readMaxBytes != 1 {
		t.Fatalf("expected maxBytes=1, got %d error=%v", filesystem.readMaxBytes, rpcErr)
	}
}

func TestReadFileToolRejectsNonPositiveMaxBytes(t *testing.T) {
	for _, maxBytes := range []string{"0", "-1"} {
		_, rpcErr := NewReadFileTool(newFakeFileSystem(), 4096).Execute(
			context.Background(), json.RawMessage(`{"path":"README.md","maxBytes":`+maxBytes+`}`),
		)
		if rpcErr == nil || errorCategory(rpcErr) != "invalid_arguments" {
			t.Fatalf("expected invalid params for maxBytes=%s, got %#v", maxBytes, rpcErr)
		}
	}
}

func TestReadFileToolMapsLimitExceeded(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.readErr = fs.ErrLimitExceeded

	tool := NewReadFileTool(filesystem, 1024)

	_, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"README.md"}`),
	)

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "limit_exceeded" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}

	if rpcErr.Error() != "filesystem error: limit exceeded" {
		t.Fatalf("expected limit message, got %q", rpcErr.Error())
	}
}

func TestReadFileToolReturnsInvalidParamsForMalformedJSON(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewReadFileTool(filesystem, 1024)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":`),
	)

	if result.variant != "" {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "invalid_arguments" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}

func TestReadFileToolReturnsInvalidParamsForMissingPath(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewReadFileTool(filesystem, 1024)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)

	if result.variant != "" {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "invalid_arguments" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}

func TestReadFileToolReturnsInvalidParamsForFilesystemError(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.readErr = fs.ErrPathIsDirectory

	tool := NewReadFileTool(filesystem, 1024)

	result, rpcErr := tool.Execute(
		context.Background(),
		json.RawMessage(`{"path":"docs"}`),
	)

	if result.variant != "" {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if errorCategory(rpcErr) != "unsupported_path_type" {
		t.Fatalf("expected ErrInvalidParams, got %s", errorCategory(rpcErr))
	}
}
