package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestCreateDirectoryDefinition(t *testing.T) {
	definition := NewCreateDirectoryTool(newFakeFileSystem()).Definition()
	if definition.Name != "create_directory" || definition.Title != "Create Directory" || definition.Description == "" || definition.InputSchema == nil {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestCreateDirectoryReportsCreatedState(t *testing.T) {
	fake := newFakeFileSystem()
	fake.mkdirCreated = true
	result, rpcErr := NewCreateDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"a/b"}`))
	if rpcErr != nil || result != (createDirectoryResult{Path: "a/b", Created: true}) {
		t.Fatalf("unexpected created result=%#v error=%v", result, rpcErr)
	}
	fake.mkdirCreated = false
	result, rpcErr = NewCreateDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"a/b"}`))
	if rpcErr != nil || result != (createDirectoryResult{Path: "a/b", Created: false}) {
		t.Fatalf("unexpected existing result=%#v error=%v", result, rpcErr)
	}
}

func TestCreateDirectoryRejectsInvalidArguments(t *testing.T) {
	for _, raw := range []string{`{}`, `{"path":""}`, `{"path":" "}`, `{"path":"a","extra":1}`} {
		_, rpcErr := NewCreateDirectoryTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || errorCategory(rpcErr) != "invalid_arguments" {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}

func TestCreateDirectoryMapsFilesystemError(t *testing.T) {
	fake := newFakeFileSystem()
	fake.mkdirErr = fs.ErrPathIsNotDirectory
	result, rpcErr := NewCreateDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"file"}`))
	if result != nil || rpcErr == nil || errorCategory(rpcErr) != "unsupported_path_type" {
		t.Fatalf("expected Invalid params, result=%#v error=%#v", result, rpcErr)
	}
}
