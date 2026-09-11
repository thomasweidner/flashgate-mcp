package tools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestGetPathInfoExistingAndMissing(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statMetadata = fs.Metadata{Name: "file.txt", Type: "file", Size: 7, ModifiedTime: "2026-09-11T10:15:30Z", Permissions: "0644"}
	tool := NewGetPathInfoTool(fake)
	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"path":"file.txt"}`))
	want := getPathInfoExistingResult{Path: "file.txt", Exists: true, Name: "file.txt", Type: "file", Size: 7, ModifiedTime: "2026-09-11T10:15:30Z", Permissions: "0644"}
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
	fake.statMetadata = fs.Metadata{Name: "docs", Type: "directory", IsDir: true, ModifiedTime: "2026-09-11T10:15:30Z"}
	result, rpcErr := NewGetPathInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"docs"}`))
	want := getPathInfoExistingResult{Path: "docs", Exists: true, Name: "docs", Type: "directory", IsDir: true, Size: 0, ModifiedTime: "2026-09-11T10:15:30Z"}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected directory result=%#v error=%v", result, rpcErr)
	}
}

func TestGetPathInfoDoesNotMaskPolicyDenial(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statErr = security.ErrHiddenPathDenied
	_, rpcErr := NewGetPathInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"path":".hidden/missing"}`))
	if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected policy error, got %#v", rpcErr)
	}
}

func TestGetPathInfoMapsFilesystemError(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statErr = fs.ErrPathIsDirectory
	result, rpcErr := NewGetPathInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"file"}`))
	if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected Invalid params, result=%#v error=%#v", result, rpcErr)
	}
}

func TestGetPathInfoRejectsInvalidArguments(t *testing.T) {
	for _, raw := range []string{`{}`, `{"path":""}`, `{"path":"  "}`, `{"path":1}`, `{"path":"a","extra":true}`, `{"path":"a"} null`} {
		_, rpcErr := NewGetPathInfoTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}

func TestMapFilesystemErrorCategories(t *testing.T) {
	cases := []struct {
		err     error
		code    int
		message string
	}{
		{fs.ErrNotFound, protocol.ErrInvalidParams, "filesystem error: not found"},
		{fs.ErrFileExists, protocol.ErrInvalidParams, "filesystem error: already exists"},
		{fs.ErrPathIsDirectory, protocol.ErrInvalidParams, "filesystem error: unsupported path type"},
		{fs.ErrCrossVolumeMoveUnsupported, protocol.ErrInvalidParams, "filesystem error: unsupported operation"},
		{fs.ErrLimitExceeded, protocol.ErrInvalidParams, "filesystem error: limit exceeded"},
		{errors.New("unexpected"), protocol.ErrInternalError, "filesystem error: io error"},
	}
	for _, tc := range cases {
		got := mapFilesystemError(tc.err)
		if got.Code != tc.code || got.Message != tc.message {
			t.Fatalf("for %v got %#v", tc.err, got)
		}
	}
}
