package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const getPathInfoToolName = "get_path_info"
const getPathsInfoToolName = "get_paths_info"
const maxPathsInfoItems = 100

// GetPathInfoTool exposes filesystem metadata lookup as an MCP tool.
type GetPathInfoTool struct{ filesystem fs.FileSystem }

func NewGetPathInfoTool(filesystem fs.FileSystem) *GetPathInfoTool {
	return &GetPathInfoTool{filesystem: filesystem}
}
func (t *GetPathInfoTool) Name() string  { return getPathInfoToolName }
func (t *GetPathInfoTool) Title() string { return "Get Path Info" }
func (t *GetPathInfoTool) Description() string {
	return "Returns existence and metadata for a file or directory below the configured filesystem root."
}
func (t *GetPathInfoTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "minLength": 1, "description": "Relative file or directory path below the configured filesystem root."},
		},
		"required": []string{"path"}, "additionalProperties": false,
	}
}
func (t *GetPathInfoTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}
func (t *GetPathInfoTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments getPathInfoArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}

	return getPathInfo(t.filesystem, arguments.Path)
}

func getPathInfo(filesystem fs.FileSystem, path string) (any, *protocol.Error) {
	metadata, err := filesystem.Stat(path)
	if errors.Is(err, fs.ErrNotFound) {
		return getPathInfoMissingResult{Path: path, Exists: false}, nil
	}
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return getPathInfoExistingResult{
		Path: path, Exists: true, Name: metadata.Name, IsDir: metadata.IsDir, Size: metadata.Size,
	}, nil
}

// GetPathsInfoTool exposes bounded, ordered filesystem metadata lookups as one MCP call.
type GetPathsInfoTool struct{ filesystem fs.FileSystem }

func NewGetPathsInfoTool(filesystem fs.FileSystem) *GetPathsInfoTool {
	return &GetPathsInfoTool{filesystem: filesystem}
}
func (t *GetPathsInfoTool) Name() string  { return getPathsInfoToolName }
func (t *GetPathsInfoTool) Title() string { return "Get Paths Info" }
func (t *GetPathsInfoTool) Description() string {
	return "Returns ordered existence and metadata results for up to 100 paths below the configured filesystem root."
}
func (t *GetPathsInfoTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paths": map[string]any{
				"type": "array", "minItems": 1, "maxItems": maxPathsInfoItems,
				"items":       map[string]any{"type": "string", "minLength": 1},
				"description": "One to 100 relative file or directory paths below the configured filesystem root.",
			},
		},
		"required": []string{"paths"}, "additionalProperties": false,
	}
}
func (t *GetPathsInfoTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}
func (t *GetPathsInfoTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments getPathsInfoArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || len(arguments.Paths) == 0 || len(arguments.Paths) > maxPathsInfoItems {
		return nil, invalidParamsError()
	}
	for _, path := range arguments.Paths {
		if !isNonBlank(path) {
			return nil, invalidParamsError()
		}
	}

	result := getPathsInfoResult{Results: make([]getPathsInfoItemResult, 0, len(arguments.Paths)), Accepted: len(arguments.Paths)}
	for _, path := range arguments.Paths {
		item, rpcErr := getPathInfo(t.filesystem, path)
		if rpcErr != nil {
			result.Results = append(result.Results, getPathsInfoItemResult{Path: path, Error: &getPathsInfoItemError{Code: pathInfoErrorCode(rpcErr), Message: rpcErr.Message}})
			result.Failed++
			continue
		}
		switch value := item.(type) {
		case getPathInfoExistingResult:
			result.Results = append(result.Results, getPathsInfoItemResult{Path: value.Path, Exists: boolPointer(true), Name: value.Name, IsDir: &value.IsDir, Size: &value.Size})
		case getPathInfoMissingResult:
			result.Results = append(result.Results, getPathsInfoItemResult{Path: value.Path, Exists: boolPointer(false)})
		}
		result.Completed++
	}
	return result, nil
}

func boolPointer(value bool) *bool { return &value }

func pathInfoErrorCode(rpcErr *protocol.Error) string {
	switch rpcErr.Message {
	case "filesystem error: not found":
		return "not_found"
	case "filesystem error: access denied":
		return "access_denied"
	case "filesystem error: unsupported path type":
		return "unsupported_path_type"
	case "filesystem error: limit exceeded":
		return "limit_exceeded"
	case "filesystem error: io error":
		return "io_error"
	default:
		return "invalid_path"
	}
}

type getPathInfoArguments struct {
	Path string `json:"path"`
}
type getPathsInfoArguments struct {
	Paths []string `json:"paths"`
}
type getPathsInfoResult struct {
	Results   []getPathsInfoItemResult `json:"results"`
	Accepted  int                      `json:"accepted"`
	Completed int                      `json:"completed"`
	Failed    int                      `json:"failed"`
}
type getPathsInfoItemResult struct {
	Path   string                 `json:"path"`
	Exists *bool                  `json:"exists,omitempty"`
	Name   string                 `json:"name,omitempty"`
	IsDir  *bool                  `json:"isDir,omitempty"`
	Size   *int64                 `json:"size,omitempty"`
	Error  *getPathsInfoItemError `json:"error,omitempty"`
}
type getPathsInfoItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type getPathInfoMissingResult struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}
type getPathInfoExistingResult struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Name   string `json:"name"`
	IsDir  bool   `json:"isDir"`
	Size   int64  `json:"size"`
}
