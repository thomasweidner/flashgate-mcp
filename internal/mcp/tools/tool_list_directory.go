package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const listDirectoryToolName = "list_directory"

// ListDirectoryTool exposes directory listing as an MCP tool.
type ListDirectoryTool struct {
	filesystem fs.FileSystem
}

// NewListDirectoryTool creates a new list_directory tool.
func NewListDirectoryTool(filesystem fs.FileSystem) *ListDirectoryTool {
	return &ListDirectoryTool{filesystem: filesystem}
}

func (t *ListDirectoryTool) Name() string  { return listDirectoryToolName }
func (t *ListDirectoryTool) Title() string { return "List Directory" }
func (t *ListDirectoryTool) Description() string {
	return "Lists files and directories below the configured filesystem root."
}
func (t *ListDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative directory path below the configured filesystem root. Defaults to '.' when omitted.",
			},
		},
		"additionalProperties": false,
	}
}
func (t *ListDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

func (t *ListDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments listDirectoryArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	path := "."
	if arguments.Path != nil {
		if !isNonBlank(*arguments.Path) {
			return nil, invalidParamsError()
		}
		path = *arguments.Path
	}

	entries, err := t.filesystem.List(path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return listDirectoryResult{Entries: entries}, nil
}

type listDirectoryArguments struct {
	Path *string `json:"path,omitempty"`
}

type listDirectoryResult struct {
	Entries []fs.Entry `json:"entries"`
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
	categoryIOError              filesystemErrorCategory = "io_error"
	categoryInvalidArguments     filesystemErrorCategory = "invalid_arguments"
	categoryUnavailableTool      filesystemErrorCategory = "unavailable_tool"
	categoryInternalError        filesystemErrorCategory = "internal_error"
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
		return categoryIOError
	}
}

func mapFilesystemError(err error) *protocol.Error {
	category := classifyFilesystemError(err)
	code := protocol.ErrInvalidParams
	if category == categoryIOError {
		code = protocol.ErrInternalError
	}

	return &protocol.Error{
		Code:    code,
		Message: "filesystem error: " + strings.ReplaceAll(string(category), "_", " "),
		Data:    toolErrorData(category),
	}
}

func toolErrorData(category filesystemErrorCategory) json.RawMessage {
	data, _ := json.Marshal(struct {
		Category filesystemErrorCategory `json:"category"`
	}{Category: category})
	return data
}
