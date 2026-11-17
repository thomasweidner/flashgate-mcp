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
	return "Returns existence and metadata for a file or directory below the selected root."
}
func (t *GetPathInfoTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root_id": rootIDSchema(),
			"path":    map[string]any{"type": "string", "minLength": 1, "description": "Relative file or directory path."},
		},
		"required": []string{"root_id", "path"}, "additionalProperties": false,
	}
}
func (t *GetPathInfoTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name()), Annotations: filesystemToolAnnotations(t.Name())}
}
func (t *GetPathInfoTool) ResultContract() ResultContract {
	return metadataContract("existing", "missing")
}

func (t *GetPathInfoTool) Execute(ctx context.Context, rawArguments json.RawMessage) (ToolResult, error) {
	var path fs.Operand
	if err := decodeStrictArguments(rawArguments, map[string]any{
		"root_id": &path.RootID,
		"path":    &path.Path,
	}); err != nil {
		return ToolResult{}, err
	}
	if !validOperand(path) {
		return ToolResult{}, invalidArgumentsError()
	}
	metadata, err := t.filesystem.Stat(path)
	if errors.Is(err, fs.ErrNotFound) {
		return prepareResult(ctx, t.filesystem, path.RootID, NewToolResult("missing", getPathInfoMissingResult{Path: path.Path, Exists: false}), t.ResultContract())
	}
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return prepareResult(ctx, t.filesystem, path.RootID, NewToolResult("existing", getPathInfoExistingResult{
		Path: path.Path, Exists: true, Name: metadata.Name, IsDir: metadata.IsDir, Size: metadata.Size,
	}), t.ResultContract())
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
