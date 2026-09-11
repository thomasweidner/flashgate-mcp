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

func TestGetPathsInfoReturnsOrderedPartialResults(t *testing.T) {
	fake := newFakeFileSystem()
	fake.statFunc = func(path string) (fs.Metadata, error) {
		switch path {
		case "file.txt":
			return fs.Metadata{Name: "file.txt", Size: 7}, nil
		case "missing.txt":
			return fs.Metadata{}, fs.ErrNotFound
		default:
			return fs.Metadata{}, security.ErrHiddenPathDenied
		}
	}
	size := int64(7)
	result, rpcErr := NewGetPathsInfoTool(fake).Execute(context.Background(), json.RawMessage(`{"paths":["file.txt","missing.txt",".hidden"]}`))
	want := getPathsInfoResult{
		Results: []getPathsInfoItemResult{
			{Path: "file.txt", Exists: boolPointer(true), Name: "file.txt", IsDir: boolPointer(false), Size: &size},
			{Path: "missing.txt", Exists: boolPointer(false)},
			{Path: ".hidden", Error: &getPathsInfoItemError{Code: "invalid_path", Message: "filesystem error: invalid path"}},
		},
		Accepted: 3, Completed: 2, Failed: 1,
	}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected batch result=%#v error=%v", result, rpcErr)
	}
}

func TestGetPathsInfoRejectsInvalidArgumentsBeforeFilesystemAccess(t *testing.T) {
	tooMany := make([]string, maxPathsInfoItems+1)
	for index := range tooMany {
		tooMany[index] = "file.txt"
	}
	rawTooMany, err := json.Marshal(map[string]any{"paths": tooMany})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{}`), json.RawMessage(`{"paths":[]}`), json.RawMessage(`{"paths":[""]}`),
		json.RawMessage(`{"paths":["  "]}`), json.RawMessage(`{"paths":null}`),
		json.RawMessage(`{"paths":["a"],"extra":true}`), rawTooMany,
	} {
		fake := newFakeFileSystem()
		_, rpcErr := NewGetPathsInfoTool(fake).Execute(context.Background(), raw)
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || fake.statPath != "" {
			t.Fatalf("expected preflight rejection for %s, error=%#v statPath=%q", raw, rpcErr, fake.statPath)
		}
	}
}

func TestGetPathsInfoDefinition(t *testing.T) {
	definition := NewGetPathsInfoTool(newFakeFileSystem()).Definition()
	if definition.Name != "get_paths_info" || definition.Title != "Get Paths Info" || definition.Description == "" || definition.InputSchema == nil || definition.OutputSchema == nil {
		t.Fatalf("unexpected definition: %#v", definition)
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
