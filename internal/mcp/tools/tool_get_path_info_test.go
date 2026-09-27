package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestGetPathInfoExistingAndMissing(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statMetadata = fs.Metadata{Name: "file.txt", Size: 7}
	tool := NewGetPathInfoTool(fake)
	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"path":"file.txt"}`))
	want := getPathInfoExistingResult{Path: "file.txt", Exists: true, Name: "file.txt", Size: 7}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected existing result=%#v error=%v", result, rpcErr)
	}
	fake.statErr = fs.ErrNotFound
	result, rpcErr = tool.Execute(context.Background(), json.RawMessage(`{"path":"missing.txt"}`))
	if rpcErr != nil || !reflect.DeepEqual(result, getPathInfoMissingResult{Path: "missing.txt", Exists: false}) {
		t.Fatalf("unexpected missing result=%#v error=%v", result, rpcErr)
	}
}

func TestGetPathInfoDefinition(t *testing.T) {
	definition := NewGetPathInfoTool(newFakeFileSystem()).Definition()
	if definition.Name != "get_path_info" || definition.Title != "Get Path Info" || definition.Description == "" || definition.InputSchema == nil {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGetPathInfoExistingDirectory(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statMetadata = fs.Metadata{Name: "docs", IsDir: true}
	result, rpcErr := NewGetPathInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"docs"}`))
	want := getPathInfoExistingResult{Path: "docs", Exists: true, Name: "docs", IsDir: true, Size: 0}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected directory result=%#v error=%v", result, rpcErr)
	}
}

func TestGetPathInfoDoesNotMaskPolicyDenial(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statErr = security.ErrHiddenPathDenied
	_, rpcErr := NewGetPathInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"path":".hidden/missing"}`))
	if rpcErr == nil || errorCategory(rpcErr) != "invalid_path" {
		t.Fatalf("expected policy error, got %#v", rpcErr)
	}
}

func TestGetPathInfoMapsFilesystemError(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statErr = fs.ErrPathIsDirectory
	result, rpcErr := NewGetPathInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"file"}`))
	if result != nil || rpcErr == nil || errorCategory(rpcErr) != "unsupported_path_type" {
		t.Fatalf("expected Invalid params, result=%#v error=%#v", result, rpcErr)
	}
}

func TestGetPathInfoRejectsInvalidArguments(t *testing.T) {
	for _, raw := range []string{`{}`, `{"path":""}`, `{"path":"  "}`, `{"path":1}`, `{"path":"a","extra":true}`, `{"path":"a"} null`} {
		_, rpcErr := NewGetPathInfoTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || errorCategory(rpcErr) != "invalid_arguments" {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}
