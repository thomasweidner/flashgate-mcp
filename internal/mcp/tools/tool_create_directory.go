package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const createDirectoryToolName = "create_directory"

// CreateDirectoryTool exposes directory creation as an MCP tool.
type CreateDirectoryTool struct{ filesystem fs.FileSystem }

func NewCreateDirectoryTool(filesystem fs.FileSystem) *CreateDirectoryTool {
	return &CreateDirectoryTool{filesystem: filesystem}
}
func (t *CreateDirectoryTool) Name() string  { return createDirectoryToolName }
func (t *CreateDirectoryTool) Title() string { return "Create Directory" }
func (t *CreateDirectoryTool) Description() string {
	return "Creates a directory and any missing parents below the selected root."
}
func (t *CreateDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root_id": rootIDSchema(),
			"path":    map[string]any{"type": "string", "minLength": 1, "description": "Relative directory path."},
		},
		"required": []string{"root_id", "path"}, "additionalProperties": false,
	}
}
func (t *CreateDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name()), Annotations: filesystemToolAnnotations(t.Name())}
}
func (t *CreateDirectoryTool) ResultContract() ResultContract {
	return metadataContract(defaultVariant)
}

func (t *CreateDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (ToolResult, error) {
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
	if err := validateReceiptPaths(path.Path); err != nil {
		return ToolResult{}, err
	}

	created, err := t.filesystem.Mkdir(path)
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return NewToolResult(defaultVariant, createDirectoryResult{Path: path.Path, Created: created}), nil
}

type createDirectoryResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}
