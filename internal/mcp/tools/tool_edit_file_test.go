package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestEditFileToolExecutesExactRange(t *testing.T) {
	t.Parallel()
	fake := newFakeFileSystem()
	fake.editSize = 7
	result, rpcErr := NewEditFileTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"a.txt","startByte":2,"endByte":5,"content":"xy"}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}
	if fake.editPath != "a.txt" || fake.editStart != 2 || fake.editEnd != 5 || string(fake.editContent) != "xy" {
		t.Fatalf("unexpected edit: %#v", fake)
	}
	if got := result.(editFileResult); got.Size != 7 || !got.Edited {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestEditFileToolRejectsInvalidRanges(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"path":"a","startByte":-1,"endByte":0,"content":""}`, `{"path":"a","startByte":2,"endByte":1,"content":""}`, `{"path":"a","startByte":0,"endByte":0}`} {
		_, rpcErr := NewEditFileTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("%s: expected invalid params, got %v", raw, rpcErr)
		}
	}
}

func TestEditFileToolExecutesExpectedMatchEdit(t *testing.T) {
	t.Parallel()
	fake := newFakeFileSystem()
	fake.editSize = 9
	result, rpcErr := NewEditFileTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"a.txt","oldText":"old","newText":"new","expectedMatches":2}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if fake.editPath != "a.txt" || string(fake.editOldContent) != "old" || string(fake.editNewContent) != "new" || fake.editExpectedMatches != 2 {
		t.Fatalf("unexpected match edit: %#v", fake)
	}
	if got := result.(editFileResult); got.Size != 9 || !got.Edited {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestEditFileToolRejectsMixedIncompleteAndMismatchedMatchEdits(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"path":"a","oldText":"","newText":"x","expectedMatches":1}`,
		`{"path":"a","oldText":"old","newText":"new","expectedMatches":0}`,
		`{"path":"a","oldText":"old","newText":"new"}`,
		`{"path":"a","startByte":0,"endByte":1,"content":"x","oldText":"old","newText":"new","expectedMatches":1}`,
	} {
		_, rpcErr := NewEditFileTool(newFakeFileSystem()).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("%s: expected invalid params, got %v", raw, rpcErr)
		}
	}
	fake := newFakeFileSystem()
	fake.editErr = fs.ErrMatchCountMismatch
	_, rpcErr := NewEditFileTool(fake).Execute(context.Background(), json.RawMessage(`{"path":"a","oldText":"old","newText":"new","expectedMatches":1}`))
	if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected mismatch to fail closed, got %v", rpcErr)
	}
}
