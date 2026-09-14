package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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
	for _, raw := range []string{``, `null`, `[]`, `{`, `{"path":""}`, `{"path":"  "}`, `{"name":""}`, `{"name":"  "}`, `{"name":"dir/file"}`, `{"namePattern":"["}`, `{"name":"a","namePattern":"*"}`, `{"type":"link"}`, `{"minSizeBytes":-1}`, `{"minSizeBytes":2,"maxSizeBytes":1}`, `{"modifiedNotBefore":"yesterday"}`, `{"modifiedNotBefore":"2026-01-02T00:00:00Z","modifiedNotAfter":"2026-01-01T00:00:00Z"}`, `{"unknown":true}`, `{} {`} {
		_, rpcErr := NewSearchPathsTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
		}
	}
}

func TestSearchPathsAppliesPortableMetadataFilters(t *testing.T) {
	modified := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{
		{Name: "small.txt", Size: 2, ModifiedTime: modified},
		{Name: "wanted.txt", Size: 7, ModifiedTime: modified},
	}
	raw := json.RawMessage(`{"type":"file","minSizeBytes":7,"maxSizeBytes":7,"modifiedNotBefore":"2026-01-02T03:04:05Z","modifiedNotAfter":"2026-01-02T03:04:05+00:00"}`)
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), raw)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	want := searchPathsResult{Paths: []search.Path{{Path: "wanted.txt"}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSearchPathsMatchesLiteralFilename(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "wanted.txt"}, {Name: "other.txt"}}

	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"name":"wanted.txt"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	want := searchPathsResult{Paths: []search.Path{{Path: "wanted.txt"}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSearchPathsMatchesFilenamePattern(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "one.go"}, {Name: "two.txt"}}

	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"namePattern":"*.go"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	want := searchPathsResult{Paths: []search.Path{{Path: "one.go"}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSearchPathsMatchesLiteralText(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "wanted.txt", Size: 12}}
	fake.readContent = []byte("hello hello!")
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"text":"hello"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	want := searchContentResult{Matches: []search.LiteralMatch{{Path: "wanted.txt", ByteOffset: 0}, {Path: "wanted.txt", ByteOffset: 6}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if fake.readPath != "wanted.txt" || fake.readMaxBytes != maxSearchBytesPerFile {
		t.Fatalf("unexpected bounded read: path=%q max=%d", fake.readPath, fake.readMaxBytes)
	}
}

func TestSearchPathsMatchesRegularExpression(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "wanted.txt", Size: 13}}
	fake.readContent = []byte("item-12 item-7")
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"regex":"item-[0-9]+"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	want := searchContentResult{Matches: []search.LiteralMatch{{Path: "wanted.txt", ByteOffset: 0}, {Path: "wanted.txt", ByteOffset: 8}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSearchPathsRejectsInvalidOrConflictingRegularExpression(t *testing.T) {
	for _, raw := range []string{`{"regex":""}`, `{"regex":"["}`, `{"text":"x","regex":"x"}`, `{"regex":"x","type":"directory"}`} {
		fake := newFakeFileSystem()
		_, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || fake.listPath != "" || fake.readPath != "" {
			t.Fatalf("expected pre-traversal invalid params for %s, list=%q read=%q err=%#v", raw, fake.listPath, fake.readPath, rpcErr)
		}
	}
}

func TestSearchPathsLiteralTextRejectsDirectoryFilter(t *testing.T) {
	_, rpcErr := NewSearchPathsTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(`{"text":"x","type":"directory"}`))
	if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected invalid params, got %#v", rpcErr)
	}
}

func TestSearchPathsRejectsInvalidUTF8LiteralBeforeTraversal(t *testing.T) {
	fake := newFakeFileSystem()
	raw := json.RawMessage(append([]byte(`{"text":"`), append([]byte{0xff}, []byte(`"}`)...)...))
	_, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), raw)
	if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || fake.listPath != "" || fake.readPath != "" {
		t.Fatalf("expected pre-traversal invalid params, list=%q read=%q err=%#v", fake.listPath, fake.readPath, rpcErr)
	}
}
