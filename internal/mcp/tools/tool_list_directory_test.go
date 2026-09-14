package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestListDirectoryDefinition(t *testing.T) {
	tool := NewListDirectoryTool(newFakeFileSystem())
	definition := tool.Definition()
	if definition.Name != "list_directory" || definition.Title != "List Directory" || definition.Description == "" {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	schema := definition.InputSchema.(map[string]any)
	if schema["additionalProperties"] != false {
		t.Fatalf("expected closed schema: %#v", schema)
	}
}

func TestListDirectoryDefaultsOnlyMissingPath(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "file.txt", Size: 1}}
	result, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{}`))
	if rpcErr != nil || fake.listPath != "." {
		t.Fatalf("expected default path, got path=%q error=%v", fake.listPath, rpcErr)
	}
	want := listDirectoryResult{Entries: fake.entries}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestListDirectoryRejectsInvalidArguments(t *testing.T) {
	for _, raw := range []string{``, `null`, `[]`, `{`, `{"path":""}`, `{"path":"  "}`, `{"unknown":true}`, `{} {}`} {
		_, rpcErr := NewListDirectoryTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}

func TestListDirectoryMapsFileAndSecurityErrors(t *testing.T) {
	for _, testErr := range []error{fs.ErrPathIsNotDirectory, security.ErrPathTraversal} {
		fake := newFakeFileSystem()
		fake.err = testErr
		_, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"docs"}`))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected Invalid params for %v, got %#v", testErr, rpcErr)
		}
	}
}

func TestListDirectoryRedactsAllPolicyDenials(t *testing.T) {
	hostPath := t.TempDir()
	for _, testErr := range []error{
		security.ErrOutsideRoot,
		security.ErrHiddenPathDenied,
		security.ErrUNCPathDenied,
		security.ErrSymlinkDenied,
		security.ErrReparsePointDenied,
	} {
		rpcErr := mapFilesystemError(fmt.Errorf("%w: %s", testErr, hostPath))
		if rpcErr.Code != protocol.ErrInvalidParams || rpcErr.Message != "filesystem error: invalid path" {
			t.Fatalf("unexpected policy mapping for %v: %#v", testErr, rpcErr)
		}
		if strings.Contains(rpcErr.Message, hostPath) {
			t.Fatalf("host path leaked for %v", testErr)
		}
	}
}

func TestListDirectoryPaginatesInDeterministicOrder(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	tool := NewListDirectoryTool(fake)

	firstValue, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"path":"docs","pageSize":2}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	first := firstValue.(listDirectoryResult)
	if got := []string{first.Entries[0].Name, first.Entries[1].Name}; !reflect.DeepEqual(got, []string{"a", "b"}) || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", first)
	}

	secondValue, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":`+strconv.Quote(first.NextCursor)+`}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	second := secondValue.(listDirectoryResult)
	if len(second.Entries) != 1 || second.Entries[0].Name != "c" || second.NextCursor != "" {
		t.Fatalf("unexpected final page: %#v", second)
	}
}

func TestListDirectoryRejectsCursorMisuse(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "a"}, {Name: "b"}}
	tool := NewListDirectoryTool(fake)
	value, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":1}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	cursor := value.(listDirectoryResult).NextCursor

	for _, raw := range []string{
		`{"cursor":` + strconv.Quote(cursor) + `,"path":"."}`,
		`{"cursor":` + strconv.Quote(cursor) + `,"pageSize":2}`,
		`{"cursor":` + strconv.Quote(cursor+"x") + `}`,
	} {
		if _, rpcErr := tool.Execute(context.Background(), json.RawMessage(raw)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected cursor rejection for %s, got %#v", raw, rpcErr)
		}
	}
}

func TestListDirectoryInvalidatesCursorWhenDirectoryChanges(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "a"}, {Name: "b"}}
	tool := NewListDirectoryTool(fake)
	value, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":1}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	fake.entries[1].Size = 1
	_, rpcErr = tool.Execute(context.Background(), json.RawMessage(`{"cursor":`+strconv.Quote(value.(listDirectoryResult).NextCursor)+`}`))
	if rpcErr == nil || rpcErr.Message != "cursor_invalidated" {
		t.Fatalf("expected safe invalidation, got %#v", rpcErr)
	}
}

func TestListDirectoryCursorDoesNotSurviveToolRestart(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "a"}, {Name: "b"}}
	value, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"pageSize":1}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	_, rpcErr = NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"cursor":`+strconv.Quote(value.(listDirectoryResult).NextCursor)+`}`))
	if rpcErr == nil || rpcErr.Message != "invalid_cursor" {
		t.Fatalf("expected restart rejection, got %#v", rpcErr)
	}
}
