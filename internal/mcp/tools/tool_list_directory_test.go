package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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
	for _, raw := range []string{``, `null`, `[]`, `{`, `{"path":""}`, `{"path":"  "}`, `{"filter":null}`, `{"filter":{"type":"link"}}`, `{"filter":{"unknown":true}}`, `{"sortBy":"modified"}`, `{"sortOrder":"sideways"}`, `{"unknown":true}`, `{} {}`} {
		_, rpcErr := NewListDirectoryTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}

func TestListDirectoryFiltersAndSorts(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{
		{Name: "zeta.txt", Size: 5},
		{Name: "alpha.md", Size: 9},
		{Name: "alpha.txt", Size: 2},
		{Name: "docs", IsDir: true, Size: 12},
	}
	result, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{
		"filter":{"namePrefix":"alpha","nameSuffix":".txt","type":"file"},
		"sortBy":"size","sortOrder":"descending"
	}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}
	want := listDirectoryResult{Entries: []fs.Entry{{Name: "alpha.txt", Size: 2}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected filtered result: %#v", result)
	}
}

func TestListDirectorySortsDeterministically(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{
		{Name: "large", Size: 10},
		{Name: "beta", Size: 2},
		{Name: "alpha", Size: 2},
		{Name: "folder", IsDir: true, Size: 1},
	}
	result, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"sortBy":"size","sortOrder":"ascending"}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}
	want := []fs.Entry{{Name: "folder", IsDir: true, Size: 1}, {Name: "alpha", Size: 2}, {Name: "beta", Size: 2}, {Name: "large", Size: 10}}
	if !reflect.DeepEqual(result.(listDirectoryResult).Entries, want) {
		t.Fatalf("unexpected order: %#v", result)
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
