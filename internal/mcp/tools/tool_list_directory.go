package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
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
			"filter": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"namePrefix": map[string]any{"type": "string"},
					"nameSuffix": map[string]any{"type": "string"},
					"type": map[string]any{
						"type": "string",
						"enum": []string{"file", "directory"},
					},
				},
			},
			"sortBy": map[string]any{
				"type": "string",
				"enum": []string{"name", "type", "size"},
			},
			"sortOrder": map[string]any{
				"type": "string",
				"enum": []string{"ascending", "descending"},
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
	if !arguments.valid() {
		return nil, invalidParamsError()
	}

	entries, err := t.filesystem.List(path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	entries = filterListDirectoryEntries(entries, arguments.Filter)
	sortListDirectoryEntries(entries, arguments.SortBy, arguments.SortOrder)

	return listDirectoryResult{Entries: entries}, nil
}

type listDirectoryArguments struct {
	Path      *string              `json:"path,omitempty"`
	Filter    *listDirectoryFilter `json:"filter,omitempty"`
	SortBy    *string              `json:"sortBy,omitempty"`
	SortOrder *string              `json:"sortOrder,omitempty"`
}

type listDirectoryFilter struct {
	NamePrefix *string `json:"namePrefix,omitempty"`
	NameSuffix *string `json:"nameSuffix,omitempty"`
	Type       *string `json:"type,omitempty"`
}

func (a listDirectoryArguments) valid() bool {
	if a.SortBy != nil && *a.SortBy != "name" && *a.SortBy != "type" && *a.SortBy != "size" {
		return false
	}
	if a.SortOrder != nil && *a.SortOrder != "ascending" && *a.SortOrder != "descending" {
		return false
	}
	if a.Filter != nil && a.Filter.Type != nil && *a.Filter.Type != "file" && *a.Filter.Type != "directory" {
		return false
	}
	return true
}

func filterListDirectoryEntries(entries []fs.Entry, filter *listDirectoryFilter) []fs.Entry {
	if filter == nil {
		return entries
	}
	filtered := make([]fs.Entry, 0, len(entries))
	for _, entry := range entries {
		if filter.NamePrefix != nil && !strings.HasPrefix(entry.Name, *filter.NamePrefix) {
			continue
		}
		if filter.NameSuffix != nil && !strings.HasSuffix(entry.Name, *filter.NameSuffix) {
			continue
		}
		if filter.Type != nil && ((*filter.Type == "directory") != entry.IsDir) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func sortListDirectoryEntries(entries []fs.Entry, sortBy, sortOrder *string) {
	field := "name"
	if sortBy != nil {
		field = *sortBy
	}
	descending := sortOrder != nil && *sortOrder == "descending"
	sort.SliceStable(entries, func(i, j int) bool {
		comparison := compareListDirectoryEntries(entries[i], entries[j], field)
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

func compareListDirectoryEntries(left, right fs.Entry, field string) int {
	comparison := 0
	switch field {
	case "type":
		comparison = strings.Compare(listDirectoryEntryType(left), listDirectoryEntryType(right))
	case "size":
		if left.Size < right.Size {
			comparison = -1
		} else if left.Size > right.Size {
			comparison = 1
		}
	default:
		comparison = strings.Compare(left.Name, right.Name)
	}
	if comparison == 0 {
		comparison = strings.Compare(left.Name, right.Name)
	}
	return comparison
}

func listDirectoryEntryType(entry fs.Entry) string {
	if entry.IsDir {
		return "directory"
	}
	return "file"
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
