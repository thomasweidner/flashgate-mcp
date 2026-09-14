package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const getPathInfoToolName = "get_path_info"

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

	metadata, err := t.filesystem.Stat(arguments.Path)
	if errors.Is(err, fs.ErrNotFound) {
		return getPathInfoMissingResult{Path: arguments.Path, Exists: false}, nil
	}
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return getPathInfoExistingResult{
		Path: arguments.Path, Exists: true, Name: metadata.Name, Type: metadata.Type, IsDir: metadata.IsDir,
		Size: metadata.Size, ModifiedTime: metadata.ModifiedTime, Permissions: metadata.Permissions,
	}, nil
}

type getPathInfoArguments struct {
	Path string `json:"path"`
}
type getPathInfoMissingResult struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}
type getPathInfoExistingResult struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsDir        bool   `json:"isDir"`
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modifiedTime"`
	Permissions  string `json:"permissions,omitempty"`
}
