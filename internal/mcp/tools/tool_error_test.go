package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func errorCategory(err error) filesystemErrorCategory {
	var expected *toolExecutionError
	if errors.As(err, &expected) && expected != nil {
		return expected.category
	}
	return ""
}
func TestExpectedFilesystemErrorsAndRawFailures(t *testing.T) {
	cases := []struct {
		cause    error
		category filesystemErrorCategory
	}{
		{fs.ErrNotFound, categoryNotFound}, {os.ErrNotExist, categoryNotFound},
		{fs.ErrFileExists, categoryAlreadyExists}, {os.ErrExist, categoryAlreadyExists},
		{os.ErrPermission, categoryAccessDenied},
		{fs.ErrPathIsDirectory, categoryUnsupportedPathType}, {fs.ErrPathIsNotDirectory, categoryUnsupportedPathType},
		{fs.ErrCopyDirectoryUnsupported, categoryUnsupportedPathType}, {fs.ErrMoveTypeMismatch, categoryUnsupportedPathType},
		{fs.ErrDirectoryNotEmpty, categoryUnsupportedPathType},
		{fs.ErrCrossVolumeMoveUnsupported, categoryUnsupportedOperation},
		{fs.ErrFileTooLarge, categoryLimitExceeded}, {fs.ErrLimitExceeded, categoryLimitExceeded},
		{fs.ErrSamePath, categoryInvalidPath}, {fs.ErrMoveIntoSelf, categoryInvalidPath}, {fs.ErrMovePathChanged, categoryInvalidPath},
	}
	for _, tc := range cases {
		raw := fmt.Errorf("private-root OS provider token: %w", tc.cause)
		got := mapFilesystemError(raw)
		if errorCategory(got) != tc.category || strings.Contains(got.Error(), "private-root") {
			t.Fatalf("unsafe mapping: %v", got)
		}
	}
	raw := errors.New("private-root OS provider credential")
	if mapFilesystemError(raw) != raw {
		t.Fatal("unexpected cause must remain internal")
	}
	for _, cause := range []error{raw, fmt.Errorf("wrapped: %w", raw), (*toolExecutionError)(nil), &toolExecutionError{}} {
		registry := NewRegistry()
		registry.Register(&testTool{name: "test_tool", err: cause})
		result, rpcErr := NewCallHandler(registry).Handle(handlers.Context{}, json.RawMessage(`{"name":"test_tool","arguments":{}}`))
		if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInternalError || rpcErr.Message != "internal error" {
			t.Fatalf("unsafe internal boundary: %v %#v", result, rpcErr)
		}
	}
}
