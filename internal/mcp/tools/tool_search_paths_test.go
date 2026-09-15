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

func TestSearchPathsExplicitlyAppliesIgnoreFile(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: ".gitignore", Size: 6}, {Name: "keep.go"}, {Name: "skip.log"}}
	fake.readContent = []byte("*.log\n")
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"ignoreFile":".gitignore"}`))
	want := searchPathsResult{Paths: []search.Path{{Path: ".gitignore"}, {Path: "keep.go"}}}
	if rpcErr != nil || !reflect.DeepEqual(result, want) || fake.readPath != ".gitignore" || fake.readMaxBytes != search.MaxIgnoreFileBytes {
		t.Fatalf("unexpected ignore result=%#v read=%q max=%d err=%#v", result, fake.readPath, fake.readMaxBytes, rpcErr)
	}
}

func TestSearchPathsPaginatesPathsWithSingleUseOpaqueCursor(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "c.txt"}, {Name: "a.txt"}, {Name: "b.txt"}}
	tool := NewSearchPathsTool(fake)

	first, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":2}`))
	firstPage, ok := first.(searchPathsResult)
	if rpcErr != nil || !ok || len(firstPage.Paths) != 2 || firstPage.Paths[0].Path != "a.txt" || firstPage.Paths[1].Path != "b.txt" || firstPage.NextCursor == "" {
		t.Fatalf("unexpected first page=%#v err=%#v", first, rpcErr)
	}
	second, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":"`+firstPage.NextCursor+`"}`))
	want := searchPathsResult{Paths: []search.Path{{Path: "c.txt"}}}
	if rpcErr != nil || !reflect.DeepEqual(second, want) {
		t.Fatalf("unexpected second page=%#v err=%#v", second, rpcErr)
	}
	if _, replayErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":"`+firstPage.NextCursor+`"}`)); replayErr == nil || replayErr.Message != "search error: stale cursor" {
		t.Fatalf("expected replay rejection, got %#v", replayErr)
	}
}

func TestSearchPathsPaginatesContentWithoutChangingMatchLimitOutcome(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "wanted.txt", Size: 3}}
	fake.readContent = []byte("xxx")
	tool := NewSearchPathsTool(fake)

	first, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"text":"x","maxMatches":2,"pageSize":1}`))
	firstPage := first.(search.ContentSearchResult)
	if rpcErr != nil || len(firstPage.Matches) != 1 || firstPage.NextCursor == "" || !firstPage.Truncated || firstPage.Limit == nil {
		t.Fatalf("unexpected first page=%#v err=%#v", first, rpcErr)
	}
	second, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":"`+firstPage.NextCursor+`"}`))
	secondPage := second.(search.ContentSearchResult)
	if rpcErr != nil || len(secondPage.Matches) != 1 || secondPage.Matches[0].ByteOffset != 1 || secondPage.NextCursor != "" || !secondPage.Truncated || secondPage.Limit == nil {
		t.Fatalf("unexpected second page=%#v err=%#v", second, rpcErr)
	}
}

func TestSearchPathsRejectsExpiredOrQueryBearingCursor(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "a"}, {Name: "b"}}
	tool := NewSearchPathsTool(fake)
	now := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
	tool.cursors.now = func() time.Time { return now }
	tool.cursors.ttl = time.Second
	first, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":1}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	cursor := first.(searchPathsResult).NextCursor
	if _, mixedErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":"`+cursor+`","path":"."}`)); mixedErr == nil {
		t.Fatal("expected query-bearing cursor rejection")
	}
	now = now.Add(time.Second)
	if _, expiredErr := tool.Execute(context.Background(), json.RawMessage(`{"cursor":"`+cursor+`"}`)); expiredErr == nil || expiredErr.Message != "search error: stale cursor" {
		t.Fatalf("expected expired cursor rejection, got %#v", expiredErr)
	}
}

func TestSearchPathsRejectsInvalidArguments(t *testing.T) {
	for _, raw := range []string{``, `null`, `[]`, `{`, `{"path":""}`, `{"path":"  "}`, `{"name":""}`, `{"name":"  "}`, `{"name":"dir/file"}`, `{"namePattern":"["}`, `{"name":"a","namePattern":"*"}`, `{"type":"link"}`, `{"minSizeBytes":-1}`, `{"minSizeBytes":2,"maxSizeBytes":1}`, `{"modifiedNotBefore":"yesterday"}`, `{"modifiedNotBefore":"2026-01-02T00:00:00Z","modifiedNotAfter":"2026-01-01T00:00:00Z"}`, `{"pageSize":0}`, `{"pageSize":101}`, `{"cursor":""}`, `{"ignoreFile":""}`, `{"ignoreFile":"  "}`, `{"unknown":true}`, `{} {`} {
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
	want := search.ContentSearchResult{Matches: []search.LiteralMatch{{Path: "wanted.txt", ByteOffset: 0}, {Path: "wanted.txt", ByteOffset: 6}}}
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
	want := search.ContentSearchResult{Matches: []search.LiteralMatch{{Path: "wanted.txt", ByteOffset: 0}, {Path: "wanted.txt", ByteOffset: 8}}}
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

func TestSearchPathsAppliesAndValidatesClientMatchLimits(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "wanted.txt", Size: 3}}
	fake.readContent = []byte("xxx")
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"text":"x","maxMatchesPerFile":2,"maxMatches":10}`))
	want := search.ContentSearchResult{Matches: []search.LiteralMatch{{Path: "wanted.txt", ByteOffset: 0}, {Path: "wanted.txt", ByteOffset: 1}}, Truncated: true, Limit: &search.MatchLimitDiagnostic{Kind: "perFileMatches", MaxMatches: 2, ReturnedMatches: 2, Path: "wanted.txt"}}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected bounded result=%#v err=%#v", result, rpcErr)
	}

	for _, raw := range []string{`{"maxMatches":1}`, `{"text":"x","maxMatches":0}`, `{"text":"x","maxMatches":1001}`, `{"regex":"x","maxMatchesPerFile":257}`} {
		fake := newFakeFileSystem()
		if _, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(raw)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || fake.listPath != "" {
			t.Fatalf("expected pre-traversal rejection for %s, err=%#v", raw, rpcErr)
		}
	}
}

func TestSearchPathsAppliesAndValidatesContextLines(t *testing.T) {
	fake := newFakeFileSystem()
	fake.entries = []fs.Entry{{Name: "wanted.txt", Size: 22}}
	fake.readContent = []byte("before\nhello\nafter\n")
	result, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(`{"text":"hello","contextLines":1}`))
	contextText := "before\nhello\nafter\n"
	want := search.ContentSearchResult{Matches: []search.LiteralMatch{{Path: "wanted.txt", ByteOffset: 7, Context: &contextText}}}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected context result=%#v err=%#v", result, rpcErr)
	}

	for _, raw := range []string{`{"contextLines":1}`, `{"text":"x","contextLines":0}`, `{"regex":"x","contextLines":11}`} {
		fake := newFakeFileSystem()
		if _, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(raw)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || fake.listPath != "" {
			t.Fatalf("expected pre-traversal rejection for %s, err=%#v", raw, rpcErr)
		}
	}
}

func TestSearchPathsValidatesBinaryAndEncodingModes(t *testing.T) {
	for _, raw := range []string{`{"binaryMode":"skip"}`, `{"text":"x","binaryMode":"unknown"}`, `{"text":"x","encoding":"utf-16"}`, `{"text":"x","binaryMode":"explicit","contextLines":1}`} {
		fake := newFakeFileSystem()
		if _, rpcErr := NewSearchPathsTool(fake).Execute(context.Background(), json.RawMessage(raw)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || fake.listPath != "" {
			t.Fatalf("expected pre-traversal rejection for %s, err=%#v", raw, rpcErr)
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

func TestSearchPathsMapsTraversalLimitToSafePublicError(t *testing.T) {
	rpcErr := mapSearchError(search.ErrTraversalLimitExceeded)
	if rpcErr.Code != protocol.ErrInvalidParams || rpcErr.Message != "search error: limit exceeded" {
		t.Fatalf("unexpected traversal-limit mapping: %#v", rpcErr)
	}
}
