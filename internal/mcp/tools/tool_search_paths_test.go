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
	"github.com/thomasweidner/flashgate-mcp/internal/search"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestSearchPathsDefinition(t *testing.T) {
	definition := NewSearchPathsTool(newFakeFileSystem()).Definition()
	if definition.Name != "search_paths" || definition.Title != "Search Paths" || definition.Description == "" {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	schema := definition.InputSchema.(map[string]any)
	if schema["additionalProperties"] != false {
		t.Fatalf("expected closed schema: %#v", schema)
	}
}

func TestSearchPathsRedactsPolicyErrors(t *testing.T) {
	hostPath := t.TempDir()
	fake := newFakeFileSystem()
	fake.err = fmt.Errorf("%w: %s", security.ErrOutsideRoot, hostPath)

	_, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{}`))
	if rpcErr == nil || rpcErr.Message != "filesystem error: invalid path" || strings.Contains(rpcErr.Message, hostPath) {
		t.Fatalf("unexpected policy error: %#v", rpcErr)
	}
}

func TestSearchPathsDefaultsOnlyMissingPath(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "file.txt"}}
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{}`))
	if rpcErr != nil || fake.listPath != "." {
		t.Fatalf("expected root search, got path=%q error=%v", fake.listPath, rpcErr)
	}
	want := searchPathsResult{Paths: []search.Path{{Path: "file.txt"}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSearchPathsRejectsInvalidArguments(t *testing.T) {
	for _, raw := range []string{``, `null`, `[]`, `{`, `{"path":""}`, `{"path":"  "}`, `{"unknown":true}`, `{} {}`} {
		_, rpcErr := NewSearchPathsTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}
