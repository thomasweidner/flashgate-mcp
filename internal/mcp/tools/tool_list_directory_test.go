package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
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
		if rpcErr == nil || errorCategory(rpcErr) != "invalid_arguments" {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}

func TestListDirectoryMapsFileAndSecurityErrors(t *testing.T) {
	for _, testErr := range []error{fs.ErrPathIsNotDirectory, security.ErrPathTraversal} {
		fake := newFakeFileSystem()
		fake.err = testErr
		_, rpcErr := NewListDirectoryTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"docs"}`))
		if rpcErr == nil || errorCategory(rpcErr) != classifyFilesystemError(testErr) {
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
		if errorCategory(rpcErr) != "invalid_path" || rpcErr.Error() != "filesystem error: invalid path" {
			t.Fatalf("unexpected policy mapping for %v: %#v", testErr, rpcErr)
		}
		if strings.Contains(rpcErr.Error(), hostPath) {
			t.Fatalf("host path leaked for %v", testErr)
		}
	}
}
