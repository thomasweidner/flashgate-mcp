package tools

import (
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestFilesystemToolAnnotationMatrix(t *testing.T) {
	t.Parallel()

	expected := map[string]protocol.ToolAnnotations{
		listDirectoryToolName:   {ReadOnlyHint: true, IdempotentHint: true},
		readFileToolName:        {ReadOnlyHint: true, IdempotentHint: true},
		getPathInfoToolName:     {ReadOnlyHint: true, IdempotentHint: true},
		writeFileToolName:       {DestructiveHint: true},
		createDirectoryToolName: {IdempotentHint: true},
		deletePathToolName:      {DestructiveHint: true, IdempotentHint: true},
		copyPathToolName:        {DestructiveHint: true},
		movePathToolName:        {DestructiveHint: true, IdempotentHint: true},
	}

	if !reflect.DeepEqual(filesystemToolAnnotationMatrix, expected) {
		t.Fatalf("annotation matrix=%#v, want %#v", filesystemToolAnnotationMatrix, expected)
	}
}

func TestFilesystemToolAnnotationsRejectsUnknownTool(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected unknown tool annotation lookup to panic")
		}
	}()
	_ = filesystemToolAnnotations("unknown_tool")
}
