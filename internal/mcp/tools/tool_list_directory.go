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

const (
	listDirectoryFieldName  = "name"
	listDirectoryFieldIsDir = "isDir"
	listDirectoryFieldSize  = "size"
)

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
			"fields": map[string]any{
				"type":        "array",
				"minItems":    1,
				"uniqueItems": true,
				"items": map[string]any{
					"type": "string",
					"enum": []string{listDirectoryFieldName, listDirectoryFieldIsDir, listDirectoryFieldSize},
				},
				"description": "Portable entry fields to return. Defaults to name, isDir, and size when omitted.",
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

	fields, ok := selectedListDirectoryFields(arguments.Fields)
	if !ok {
		return nil, invalidParamsError()
	}

	entries, err := t.filesystem.List(path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return listDirectoryResult{Entries: projectListDirectoryEntries(entries, fields)}, nil
}

type listDirectoryArguments struct {
	Path   *string  `json:"path,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

type listDirectoryResult struct {
	Entries []listDirectoryEntry `json:"entries"`
}

type listDirectoryEntry struct {
	Name  *string `json:"name,omitempty"`
	IsDir *bool   `json:"isDir,omitempty"`
	Size  *int64  `json:"size,omitempty"`
}

type listDirectoryFields struct {
	name  bool
	isDir bool
	size  bool
}

func selectedListDirectoryFields(requested []string) (listDirectoryFields, bool) {
	if requested == nil {
		return listDirectoryFields{name: true, isDir: true, size: true}, true
	}
	if len(requested) == 0 {
		return listDirectoryFields{}, false
	}

	var selected listDirectoryFields
	for _, field := range requested {
		switch field {
		case listDirectoryFieldName:
			if selected.name {
				return listDirectoryFields{}, false
			}
			selected.name = true
		case listDirectoryFieldIsDir:
			if selected.isDir {
				return listDirectoryFields{}, false
			}
			selected.isDir = true
		case listDirectoryFieldSize:
			if selected.size {
				return listDirectoryFields{}, false
			}
			selected.size = true
		default:
			return listDirectoryFields{}, false
		}
	}
	return selected, true
}

func projectListDirectoryEntry(entry fs.Entry, fields listDirectoryFields) listDirectoryEntry {
	var result listDirectoryEntry
	if fields.name {
		result.Name = &entry.Name
	}
	if fields.isDir {
		result.IsDir = &entry.IsDir
	}
	if fields.size {
		result.Size = &entry.Size
	}
	return result
}

func projectListDirectoryEntries(entries []fs.Entry, fields listDirectoryFields) []listDirectoryEntry {
	result := make([]listDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, projectListDirectoryEntry(entry, fields))
	}
	return result
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

	return &protocol.Error{Code: code, Message: "filesystem error: " + strings.ReplaceAll(string(category), "_", " ")}
}
