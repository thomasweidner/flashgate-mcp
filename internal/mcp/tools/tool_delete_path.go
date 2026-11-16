package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const deletePathToolName = "delete_path"

// DeletePathTool exposes filesystem deletion as an MCP tool.
type DeletePathTool struct {
	filesystem fs.FileSystem
}

// NewDeletePathTool creates a new delete_path tool.
func NewDeletePathTool(filesystem fs.FileSystem) *DeletePathTool {
	return &DeletePathTool{
		filesystem: filesystem,
	}
}

// Name returns the tool name.
func (t *DeletePathTool) Name() string {
	return deletePathToolName
}

// Title returns the human-readable tool title.
func (t *DeletePathTool) Title() string {
	return "Delete Path"
}

// Description returns the tool description.
func (t *DeletePathTool) Description() string {
	return "Deletes a file or directory below the selected root."
}

// InputSchema returns the JSON schema for this tool.
func (t *DeletePathTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root_id": rootIDSchema(),
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative file or directory path.",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "Whether a non-empty directory may be deleted recursively. Defaults to false.",
			},
		},
		"required":             []string{"root_id", "path"},
		"additionalProperties": false,
	}
}

// Definition returns the MCP tool definition.
func (t *DeletePathTool) Definition() protocol.Tool {
	return protocol.Tool{
		Name:         t.Name(),
		Title:        t.Title(),
		Description:  t.Description(),
		InputSchema:  t.InputSchema(),
		OutputSchema: filesystemOutputSchema(t.Name()),
		Annotations:  filesystemToolAnnotations(t.Name()),
	}
}

func (t *DeletePathTool) ResultContract() ResultContract {
	return metadataContract(defaultVariant)
}

// Execute deletes the requested path.
func (t *DeletePathTool) Execute(_ context.Context, rawArguments json.RawMessage) (ToolResult, error) {
	var path fs.Operand
	var recursive bool
	if err := decodeStrictArguments(rawArguments, map[string]any{
		"root_id":   &path.RootID,
		"path":      &path.Path,
		"recursive": &recursive,
	}); err != nil {
		return ToolResult{}, err
	}
	if !validOperand(path) {
		return ToolResult{}, invalidArgumentsError()
	}
	if err := validateReceiptPaths(path.Path); err != nil {
		return ToolResult{}, err
	}

	if err := t.filesystem.Delete(path, recursive); err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return NewToolResult(defaultVariant, deletePathResult{
		Path:    path.Path,
		Deleted: true,
	}), nil
}

type deletePathResult struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}
