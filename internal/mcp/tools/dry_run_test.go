package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestFilesystemMutationToolsDryRunReturnsPreviewWithoutMutation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arguments string
		newTool   func(*fakeFileSystem) Tool
		want      map[string]any
		wantPaths []string
	}{
		{"write_file", `{"path":"out.txt","content":"data","overwrite":true,"dryRun":true}`, func(f *fakeFileSystem) Tool { return NewWriteFileTool(f) }, map[string]any{"path": "out.txt", "size": float64(4), "written": false, "dryRun": true}, []string{"out.txt"}},
		{"create_directory", `{"path":"out/archive","dryRun":true}`, func(f *fakeFileSystem) Tool { return NewCreateDirectoryTool(f) }, map[string]any{"path": "out/archive", "created": false, "dryRun": true}, []string{"out/archive"}},
		{"delete_path", `{"path":"out/archive","recursive":true,"dryRun":true}`, func(f *fakeFileSystem) Tool { return NewDeletePathTool(f) }, map[string]any{"path": "out/archive", "deleted": false, "dryRun": true}, []string{"out/archive"}},
		{"copy_path", `{"source":"a.txt","target":"b.txt","overwrite":true,"dryRun":true}`, func(f *fakeFileSystem) Tool { return NewCopyPathTool(f) }, map[string]any{"source": "a.txt", "target": "b.txt", "copied": false, "dryRun": true}, []string{"a.txt", "b.txt"}},
		{"move_path", `{"source":"a.txt","target":"b.txt","overwrite":true,"dryRun":true}`, func(f *fakeFileSystem) Tool { return NewMovePathTool(f) }, map[string]any{"source": "a.txt", "target": "b.txt", "moved": false, "dryRun": true}, []string{"a.txt", "b.txt"}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			filesystem := newFakeFileSystem()
			result, rpcErr := test.newTool(filesystem).Execute(context.Background(), json.RawMessage(test.arguments))
			if rpcErr != nil {
				t.Fatalf("Execute() error = %v", rpcErr)
			}

			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("preview = %#v, want %#v", got, test.want)
			}
			if !reflect.DeepEqual(filesystem.validatedPaths, test.wantPaths) {
				t.Fatalf("validated paths = %#v, want %#v", filesystem.validatedPaths, test.wantPaths)
			}

			if filesystem.writePath != "" || filesystem.mkdirPath != "" || filesystem.deletePath != "" || filesystem.copySource != "" || filesystem.moveSource != "" {
				t.Fatalf("dry run invoked a mutating filesystem method: %#v", filesystem)
			}
		})
	}
}
