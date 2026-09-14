package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
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

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Content string `json:"content"`
		Size    int64  `json:"size"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if decoded.Content != "hello world" {
		t.Fatalf("expected content hello world, got %q", decoded.Content)
	}

	if decoded.Size != int64(len("hello world")) {
		t.Fatalf("expected size %d, got %d", len("hello world"), decoded.Size)
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

func TestReadFileToolReadsLineWindow(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.readContent = []byte("second\nthird\n")
	tool := NewReadFileTool(filesystem, 4096)

	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(
		`{"path":"README.md","startLine":2,"endLine":3,"maxBytes":256}`,
	))
	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}
	if filesystem.readPath != "README.md" || filesystem.readStartLine != 2 || filesystem.readEndLine != 3 || filesystem.readMaxBytes != 256 || filesystem.readMaxScanBytes != 4096 {
		t.Fatalf("unexpected line read: path=%q start=%d end=%d max=%d scanMax=%d", filesystem.readPath, filesystem.readStartLine, filesystem.readEndLine, filesystem.readMaxBytes, filesystem.readMaxScanBytes)
	}
	if result != (readFileResult{Content: "second\nthird\n", Size: 13}) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReadFileToolRejectsInvalidLineWindow(t *testing.T) {
	t.Parallel()

	for _, arguments := range []string{
		`{"path":"README.md","startLine":1}`,
		`{"path":"README.md","endLine":1}`,
		`{"path":"README.md","startLine":0,"endLine":1}`,
		`{"path":"README.md","startLine":3,"endLine":2}`,
	} {
		_, rpcErr := NewReadFileTool(newFakeFileSystem(), 4096).Execute(context.Background(), json.RawMessage(arguments))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %s, got %#v", arguments, rpcErr)
		}
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
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
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

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}

	if rpcErr.Message != "filesystem error: limit exceeded" {
		t.Fatalf("expected limit message, got %q", rpcErr.Message)
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

func TestReadFileToolReturnsInvalidParamsForMissingPath(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	tool := NewReadFileTool(filesystem, 1024)

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

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
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
