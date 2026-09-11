package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestAppendFileToolDefinition(t *testing.T) {
	tool := NewAppendFileTool(newFakeFileSystem())
	definition := tool.Definition()
	if definition.Name != "append_file" || definition.Title != "Append File" {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if definition.OutputSchema == nil {
		t.Fatal("expected output schema")
	}
}

func TestAppendFileToolExecute(t *testing.T) {
	filesystem := newFakeFileSystem()
	tool := NewAppendFileTool(filesystem)

	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"path":"log.txt","content":"entry\n"}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if filesystem.appendPath != "log.txt" || string(filesystem.appendContent) != "entry\n" {
		t.Fatalf("unexpected append call: path=%q content=%q", filesystem.appendPath, filesystem.appendContent)
	}
	want := appendFileResult{Path: "log.txt", Size: 6, Appended: true}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result=%#v, want %#v", result, want)
	}
}

func TestAppendFileToolRejectsInvalidArguments(t *testing.T) {
	tool := NewAppendFileTool(newFakeFileSystem())
	for _, arguments := range []string{`{}`, `{"path":" "}`, `{"path":"file","unknown":true}`, `{"path":"file","content":null}`} {
		if _, rpcErr := tool.Execute(context.Background(), json.RawMessage(arguments)); rpcErr == nil {
			t.Fatalf("expected invalid params for %s", arguments)
		}
	}
}

func TestAppendFileToolMapsFilesystemError(t *testing.T) {
	filesystem := newFakeFileSystem()
	filesystem.appendErr = fs.ErrLimitExceeded
	if _, rpcErr := NewAppendFileTool(filesystem).Execute(context.Background(), json.RawMessage(`{"path":"log.txt","content":"x"}`)); rpcErr == nil {
		t.Fatal("expected filesystem error")
	}
}
