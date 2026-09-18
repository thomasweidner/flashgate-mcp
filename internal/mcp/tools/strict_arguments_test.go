package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestAllToolsRejectMalformedUnknownTrailingAndWrongTypes(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
	}{
		{"list_directory", NewListDirectoryTool(newFakeFileSystem())},
		{"read_file", NewReadFileTool(newFakeFileSystem(), 1024)},
		{"get_path_info", NewGetPathInfoTool(newFakeFileSystem())},
		{"write_file", NewWriteFileTool(newFakeFileSystem())},
		{"append_file", NewAppendFileTool(newFakeFileSystem())},
		{"create_directory", NewCreateDirectoryTool(newFakeFileSystem())},
		{"delete_path", NewDeletePathTool(newFakeFileSystem())},
		{"copy_path", NewCopyPathTool(newFakeFileSystem())},
		{"move_path", NewMovePathTool(newFakeFileSystem())},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, raw := range []string{`{`, `[]`, `{"unknown":true}`, `{} {}`, `{"path":1,"source":1,"target":1}`} {
				_, rpcErr := test.tool.Execute(context.Background(), json.RawMessage(raw))
				if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
					t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
				}
			}
		})
	}
}

func TestRequiredPathsRejectMissingEmptyAndWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		missing string
		empty   string
		blank   string
	}{
		{"read_file", NewReadFileTool(newFakeFileSystem(), 1024), `{}`, `{"path":""}`, `{"path":"  "}`},
		{"get_path_info", NewGetPathInfoTool(newFakeFileSystem()), `{}`, `{"path":""}`, `{"path":"  "}`},
		{"write_file", NewWriteFileTool(newFakeFileSystem()), `{}`, `{"path":""}`, `{"path":"  "}`},
		{"append_file", NewAppendFileTool(newFakeFileSystem()), `{}`, `{"path":""}`, `{"path":"  "}`},
		{"create_directory", NewCreateDirectoryTool(newFakeFileSystem()), `{}`, `{"path":""}`, `{"path":"  "}`},
		{"delete_path", NewDeletePathTool(newFakeFileSystem()), `{}`, `{"path":""}`, `{"path":"  "}`},
		{"copy_path", NewCopyPathTool(newFakeFileSystem()), `{}`, `{"source":"","target":"target"}`, `{"source":"  ","target":"target"}`},
		{"move_path", NewMovePathTool(newFakeFileSystem()), `{}`, `{"source":"source","target":""}`, `{"source":"source","target":"  "}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, raw := range []string{test.missing, test.empty, test.blank} {
				_, rpcErr := test.tool.Execute(context.Background(), json.RawMessage(raw))
				if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
					t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
				}
			}
		})
	}
}

func TestAllToolsRejectExplicitNullFields(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
		raw  []string
	}{
		{"list_directory", NewListDirectoryTool(newFakeFileSystem()), []string{`{"path":null}`}},
		{"read_file", NewReadFileTool(newFakeFileSystem(), 1024), []string{`{"path":null}`, `{"path":"file","maxBytes":null}`}},
		{"get_path_info", NewGetPathInfoTool(newFakeFileSystem()), []string{`{"path":null}`}},
		{"write_file", NewWriteFileTool(newFakeFileSystem()), []string{`{"path":"file","content":null}`, `{"path":"file","overwrite":null}`}},
		{"append_file", NewAppendFileTool(newFakeFileSystem()), []string{`{"path":"file","content":null}`}},
		{"create_directory", NewCreateDirectoryTool(newFakeFileSystem()), []string{`{"path":null}`}},
		{"delete_path", NewDeletePathTool(newFakeFileSystem()), []string{`{"path":"file","recursive":null}`}},
		{"copy_path", NewCopyPathTool(newFakeFileSystem()), []string{`{"source":"a","target":"b","overwrite":null}`}},
		{"move_path", NewMovePathTool(newFakeFileSystem()), []string{`{"source":"a","target":"b","overwrite":null}`}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, raw := range test.raw {
				_, rpcErr := test.tool.Execute(context.Background(), json.RawMessage(raw))
				if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
					t.Fatalf("expected invalid params for %q, got %#v", raw, rpcErr)
				}
			}
		})
	}
}

func TestStrictArgumentsPreserveValidPathStrings(t *testing.T) {
	fake := newFakeFileSystem()
	path := "  directory with spaces  "
	_, rpcErr := NewListDirectoryTool(fake).Execute(
		context.Background(), json.RawMessage(`{"path":"  directory with spaces  "}`),
	)
	if rpcErr != nil || fake.listPath != path {
		t.Fatalf("expected path to remain %q, got %q error=%#v", path, fake.listPath, rpcErr)
	}
}
