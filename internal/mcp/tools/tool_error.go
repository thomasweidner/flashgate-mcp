package tools

import (
	"errors"
	"os"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// toolExecutionError belongs to the MCP adapter, never the filesystem core.
type toolExecutionError struct {
	category filesystemErrorCategory
	message  string
}

func (e *toolExecutionError) Error() string { return e.message }
func invalidArgumentsError() *toolExecutionError {
	return &toolExecutionError{category: "invalid_arguments", message: "invalid arguments"}
}

type filesystemErrorCategory string

const (
	categoryNotFound             filesystemErrorCategory = "not_found"
	categoryAlreadyExists        filesystemErrorCategory = "already_exists"
	categoryAccessDenied         filesystemErrorCategory = "access_denied"
	categoryInvalidPath          filesystemErrorCategory = "invalid_path"
	categoryUnsupportedPathType  filesystemErrorCategory = "unsupported_path_type"
	categoryUnsupportedOperation filesystemErrorCategory = "unsupported_operation"
	categoryLimitExceeded        filesystemErrorCategory = "limit_exceeded"
	categoryUnknown              filesystemErrorCategory = ""
)

func classifyFilesystemError(err error) filesystemErrorCategory {
	switch {
	case errors.Is(err, security.ErrAbsolutePath),
		errors.Is(err, security.ErrPathTraversal),
		errors.Is(err, security.ErrOutsideRoot),
		errors.Is(err, security.ErrHiddenPathDenied),
		errors.Is(err, security.ErrUNCPathDenied),
		errors.Is(err, security.ErrSymlinkDenied),
		errors.Is(err, security.ErrReparsePointDenied),
		errors.Is(err, fs.ErrSamePath),
		errors.Is(err, fs.ErrMoveIntoSelf),
		errors.Is(err, fs.ErrMovePathChanged):
		return categoryInvalidPath
	case errors.Is(err, fs.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return categoryNotFound
	case errors.Is(err, fs.ErrFileExists), errors.Is(err, os.ErrExist):
		return categoryAlreadyExists
	case errors.Is(err, os.ErrPermission):
		return categoryAccessDenied
	case errors.Is(err, fs.ErrPathIsDirectory),
		errors.Is(err, fs.ErrPathIsNotDirectory),
		errors.Is(err, fs.ErrCopyDirectoryUnsupported),
		errors.Is(err, fs.ErrMoveTypeMismatch),
		errors.Is(err, fs.ErrDirectoryNotEmpty):
		return categoryUnsupportedPathType
	case errors.Is(err, fs.ErrCrossVolumeMoveUnsupported):
		return categoryUnsupportedOperation
	case errors.Is(err, fs.ErrFileTooLarge), errors.Is(err, fs.ErrLimitExceeded):
		return categoryLimitExceeded
	default:
		return categoryUnknown
	}
}

func mapFilesystemError(err error) error {
	category := classifyFilesystemError(err)
	if category == categoryUnknown {
		return err
	}
	return &toolExecutionError{category: category, message: "filesystem error: " + strings.ReplaceAll(string(category), "_", " ")}
}
