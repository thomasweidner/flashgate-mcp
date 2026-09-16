package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

// TestPerRootPolicySecurityRejectsCrossPolicyOperations is intentionally a
// boundary-level matrix: every filesystem tool must resolve the selected root
// and reject an incompatible access policy before invoking any filesystem
// method.
func TestPerRootPolicySecurityRejectsCrossPolicyOperations(t *testing.T) {
	tests := []struct {
		name   string
		access roots.Access
		tool   func(*fakeFileSystem) Tool
		input  string
	}{
		{name: "list requires read", access: roots.Write, tool: func(f *fakeFileSystem) Tool { return NewListDirectoryTool(f) }, input: `{"rootId":"restricted","path":"."}`},
		{name: "read requires read", access: roots.Write, tool: func(f *fakeFileSystem) Tool { return NewReadFileTool(f, 4096) }, input: `{"rootId":"restricted","path":"secret.txt"}`},
		{name: "info requires read", access: roots.Write, tool: func(f *fakeFileSystem) Tool { return NewGetPathInfoTool(f) }, input: `{"rootId":"restricted","path":"secret.txt"}`},
		{name: "write requires write", access: roots.Read, tool: func(f *fakeFileSystem) Tool { return NewWriteFileTool(f) }, input: `{"rootId":"restricted","path":"blocked.txt","content":"blocked"}`},
		{name: "mkdir requires write", access: roots.Read, tool: func(f *fakeFileSystem) Tool { return NewCreateDirectoryTool(f) }, input: `{"rootId":"restricted","path":"blocked"}`},
		{name: "delete requires write", access: roots.Read, tool: func(f *fakeFileSystem) Tool { return NewDeletePathTool(f) }, input: `{"rootId":"restricted","path":"blocked.txt"}`},
		{name: "copy requires read and write", access: roots.Read, tool: func(f *fakeFileSystem) Tool { return NewCopyPathTool(f) }, input: `{"rootId":"restricted","source":"source.txt","target":"target.txt"}`},
		{name: "move requires read and write", access: roots.Write, tool: func(f *fakeFileSystem) Tool { return NewMovePathTool(f) }, input: `{"rootId":"restricted","source":"source.txt","target":"target.txt"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filesystem := newFakeFileSystem()
			registry, err := roots.New([]roots.Entry{{
				ID:           "restricted",
				FileSystem:   filesystem,
				Access:       test.access,
				Limits:       roots.DefaultLimits(1024, 2048),
				FileTypes:    roots.AllFileTypes(),
				LinkRules:    roots.LinkRules{Symlinks: roots.DenySymlinks, ReparsePoints: roots.DenyReparsePoints},
				Capabilities: capabilityForAccess(test.access),
			}})
			if err != nil {
				t.Fatal(err)
			}

			tool := test.tool(filesystem)
			BindRootRegistry(tool, registry)
			result, rpcErr := tool.Execute(context.Background(), json.RawMessage(test.input))
			if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
				t.Fatalf("policy denial = (%#v, %#v), want generic invalid params", result, rpcErr)
			}
			assertNoFilesystemOperation(t, filesystem)
		})
	}
}

func assertNoFilesystemOperation(t *testing.T, filesystem *fakeFileSystem) {
	t.Helper()
	if filesystem.listPath != "" || filesystem.readPath != "" || filesystem.statPath != "" ||
		filesystem.writePath != "" || filesystem.mkdirPath != "" || filesystem.deletePath != "" ||
		filesystem.copySource != "" || filesystem.copyTarget != "" ||
		filesystem.moveSource != "" || filesystem.moveTarget != "" {
		t.Fatalf("denied operation reached filesystem: %#v", filesystem)
	}
}
