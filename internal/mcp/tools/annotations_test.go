package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestFilesystemToolAnnotations(t *testing.T) {
	t.Parallel()

	readOnly := map[string]bool{listDirectoryToolName: true, readFileToolName: true, getPathInfoToolName: true}
	destructive := map[string]bool{writeFileToolName: true, deletePathToolName: true, copyPathToolName: true, movePathToolName: true}

	if len(filesystemToolAnnotations) != 8 {
		t.Fatalf("annotation count = %d, want 8", len(filesystemToolAnnotations))
	}
	for name, annotations := range filesystemToolAnnotations {
		if annotations.ReadOnlyHint != readOnly[name] {
			t.Errorf("%s readOnlyHint = %t, want %t", name, annotations.ReadOnlyHint, readOnly[name])
		}
		if annotations.DestructiveHint != destructive[name] {
			t.Errorf("%s destructiveHint = %t, want %t", name, annotations.DestructiveHint, destructive[name])
		}
		if !annotations.IdempotentHint {
			t.Errorf("%s idempotentHint = false, want true", name)
		}
		if annotations.OpenWorldHint {
			t.Errorf("%s openWorldHint = true, want false", name)
		}

		encoded, err := json.Marshal(protocol.Tool{Annotations: annotations})
		if err != nil {
			t.Fatalf("marshal %s annotations: %v", name, err)
		}
		for _, field := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			if !strings.Contains(string(encoded), `"`+field+`"`) {
				t.Errorf("%s omitted annotation field %s: %s", name, field, encoded)
			}
		}
	}
}
