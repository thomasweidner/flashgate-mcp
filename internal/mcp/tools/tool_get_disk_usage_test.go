package tools

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestGetDiskUsageToolExecute(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.diskUsage = fs.DiskUsage{TotalBytes: 1000, UsedBytes: 400, AvailableBytes: 500}
	tool := NewGetDiskUsageTool(filesystem)

	result, rpcErr := tool.Execute(t.Context(), []byte(`{"path":"data/file.txt"}`))
	if rpcErr != nil {
		t.Fatalf("unexpected protocol error: %#v", rpcErr)
	}
	if filesystem.diskUsagePath != "data/file.txt" {
		t.Fatalf("got provider path %q", filesystem.diskUsagePath)
	}
	want := getDiskUsageResult{Path: "data/file.txt", TotalBytes: 1000, UsedBytes: 400, AvailableBytes: 500}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGetDiskUsageToolRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tool := NewGetDiskUsageTool(newFakeFileSystem())
	for _, arguments := range []string{`{}`, `{"path":""}`, `{"path":"   "}`, `{"path":".","extra":true}`, `{"path":null}`} {
		if _, rpcErr := tool.Execute(t.Context(), []byte(arguments)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("arguments %s: expected Invalid params, got %#v", arguments, rpcErr)
		}
	}
}

func TestGetDiskUsageToolMapsFilesystemErrors(t *testing.T) {
	t.Parallel()

	filesystem := newFakeFileSystem()
	filesystem.diskUsageErr = fs.ErrNotFound
	tool := NewGetDiskUsageTool(filesystem)

	if _, rpcErr := tool.Execute(t.Context(), []byte(`{"path":"missing"}`)); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected safe filesystem error, got %#v", rpcErr)
	}
	filesystem.diskUsageErr = errors.New("host detail")
	if _, rpcErr := tool.Execute(t.Context(), []byte(`{"path":"data"}`)); rpcErr == nil || rpcErr.Code != protocol.ErrInternalError {
		t.Fatalf("expected internal filesystem error, got %#v", rpcErr)
	}
}
