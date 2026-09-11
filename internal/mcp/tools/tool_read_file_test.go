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

func TestReadFileToolAcceptsOneByteLimit(t *testing.T) {
	filesystem := newFakeFileSystem()
	_, rpcErr := NewReadFileTool(filesystem, 4096).Execute(context.Background(), json.RawMessage(`{"path":"README.md","maxBytes":1}`))
	if rpcErr != nil || filesystem.readMaxBytes != 1 {
		t.Fatalf("expected maxBytes=1, got %d error=%v", filesystem.readMaxBytes, rpcErr)
	}
}

func TestReadFileToolByteRanges(t *testing.T) {
	for _, tc := range []struct {
		name           string
		args           string
		offset, length int64
	}{
		{"offset and length", `{"path":"file","byteOffset":7,"byteLength":3}`, 7, 3},
		{"head", `{"path":"file","headBytes":5}`, 0, 5},
		{"tail", `{"path":"file","tailBytes":4}`, -4, 4},
		{"empty head", `{"path":"file","headBytes":0}`, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filesystem := newFakeFileSystem()
			if _, rpcErr := NewReadFileTool(filesystem, 10).Execute(context.Background(), json.RawMessage(tc.args)); rpcErr != nil {
				t.Fatalf("Execute: %v", rpcErr)
			}
			if filesystem.readRangeOffset != tc.offset || filesystem.readRangeLength != tc.length {
				t.Fatalf("range=(%d,%d), want (%d,%d)", filesystem.readRangeOffset, filesystem.readRangeLength, tc.offset, tc.length)
			}
		})
	}
}

func TestReadFileToolRejectsInvalidByteRanges(t *testing.T) {
	for _, args := range []string{
		`{"path":"file","byteOffset":1}`,
		`{"path":"file","byteLength":1}`,
		`{"path":"file","byteOffset":-1,"byteLength":1}`,
		`{"path":"file","headBytes":1,"tailBytes":1}`,
		`{"path":"file","byteOffset":0,"byteLength":1,"headBytes":1}`,
		`{"path":"file","headBytes":11}`,
	} {
		if _, rpcErr := NewReadFileTool(newFakeFileSystem(), 10).Execute(context.Background(), json.RawMessage(args)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %s, got %#v", args, rpcErr)
		}
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
