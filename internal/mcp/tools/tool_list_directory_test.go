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
	fields := schema["properties"].(map[string]any)["fields"].(map[string]any)
	if fields["minItems"] != 1 || fields["uniqueItems"] != true {
		t.Fatalf("expected bounded unique field selection: %#v", fields)
	}
}

func TestListDirectoryDefaultsOnlyMissingPath(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "file.txt", Size: 1}}
	result, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{}`))
	if rpcErr != nil || fake.listPath != "." {
		t.Fatalf("expected default path, got path=%q error=%v", fake.listPath, rpcErr)
	}
	want := listDirectoryResult{Entries: []listDirectoryEntry{{Name: stringPointer("file.txt"), IsDir: boolPointer(false), Size: int64Pointer(1)}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestListDirectorySelectsOnlyRequestedFields(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "directory", IsDir: true, Size: 42}}

	result, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"fields":["name","isDir"]}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"entries":[{"name":"directory","isDir":true}]}` {
		t.Fatalf("unexpected projected result: %s", raw)
	}
}

func TestListDirectoryRejectsInvalidFieldSelectionsBeforeListing(t *testing.T) {
	for _, raw := range []string{`{"fields":[]}`, `{"fields":["unknown"]}`, `{"fields":["name","name"]}`} {
		fake := newFakeFileSystem()
		_, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %s, got %#v", raw, rpcErr)
		}
		if fake.listPath != "" {
			t.Fatalf("filesystem listing occurred for invalid fields %s", raw)
		}
	}
}

func stringPointer(value string) *string { return &value }
func boolPointer(value bool) *bool       { return &value }
func int64Pointer(value int64) *int64    { return &value }

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
