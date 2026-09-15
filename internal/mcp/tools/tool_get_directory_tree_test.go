package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func newTreeTestTool(t *testing.T) (*GetDirectoryTreeTool, string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{"a.txt": "a", "dir/b.txt": "bb", "dir/sub/c.txt": "ccc"} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	filesystem, err := fs.NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	return NewGetDirectoryTreeTool(filesystem), root
}

func TestGetDirectoryTreeDepthFieldsAndOrder(t *testing.T) {
	tool, _ := newTreeTestTool(t)
	value, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"maxDepth":2,"fields":["isDir"]}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
	result := value.(directoryTreeResult)
	if result.Truncated || result.NextCursor != "" || len(result.Entries) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []string{"a.txt", "dir", "dir/b.txt", "dir/sub"}
	for i, entry := range result.Entries {
		if entry.Path != want[i] || entry.Name != "" || entry.IsDir == nil || entry.Size != nil {
			t.Fatalf("entry %d: %#v", i, entry)
		}
	}
}

func TestGetDirectoryTreePaginationAndInvalidation(t *testing.T) {
	tool, root := newTreeTestTool(t)
	value, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"maxDepth":3,"pageSize":2}`))
	if rpcErr != nil {
		t.Fatalf("first page: %#v", rpcErr)
	}
	first := value.(directoryTreeResult)
	if len(first.Entries) != 2 || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	value, rpcErr = tool.Execute(context.Background(), json.RawMessage(`{"cursor":`+quoteJSON(first.NextCursor)+`}`))
	if rpcErr != nil {
		t.Fatalf("second page: %#v", rpcErr)
	}
	if got := value.(directoryTreeResult); len(got.Entries) != 2 || got.Entries[0].Path != "dir/b.txt" {
		t.Fatalf("unexpected second page: %#v", got)
	}

	firstValue, _ := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":1}`))
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":`+quoteJSON(firstValue.(directoryTreeResult).NextCursor)+`}`)); rpcErr == nil || rpcErr.Message != "cursor_invalidated" {
		t.Fatalf("expected cursor invalidation, got %#v", rpcErr)
	}
}

func TestGetDirectoryTreeEntryAndByteBounds(t *testing.T) {
	tool, _ := newTreeTestTool(t)
	value, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"maxEntries":2,"pageSize":10,"maxBytes":512}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
	result := value.(directoryTreeResult)
	if len(result.Entries) == 0 || !result.Truncated {
		t.Fatalf("expected bounded truncated result: %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > 512 {
		t.Fatalf("encoded bytes=%d err=%v", len(encoded), err)
	}
}

func TestGetDirectoryTreeRejectsInvalidControlsAndCursorConflicts(t *testing.T) {
	tool, _ := newTreeTestTool(t)
	for _, raw := range []string{`{"maxDepth":33}`, `{"maxEntries":0}`, `{"maxBytes":511}`, `{"fields":[]}`, `{"fields":["name","name"]}`, `{"fields":["owner"]}`} {
		if _, rpcErr := tool.Execute(context.Background(), json.RawMessage(raw)); rpcErr == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
	value, _ := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":1}`))
	cursor := value.(directoryTreeResult).NextCursor
	if _, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":`+quoteJSON(cursor)+`,"path":"."}`)); rpcErr == nil {
		t.Fatal("expected cursor/query conflict rejection")
	}
}

func quoteJSON(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
