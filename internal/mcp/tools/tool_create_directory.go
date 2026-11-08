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
	return "Creates a directory and any missing parents below the configured filesystem root."
}
func (t *CreateDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "minLength": 1, "description": "Relative directory path below the configured filesystem root."},
		},
		"required": []string{"path"}, "additionalProperties": false,
	}
}
func (t *CreateDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name()), Annotations: filesystemToolAnnotations(t.Name())}
}
func (t *CreateDirectoryTool) ResultContract() ResultContract {
	return metadataContract(defaultVariant)
}

func (t *CreateDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (ToolResult, error) {
	var arguments createDirectoryArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || !isNonBlank(arguments.Path) {
		return ToolResult{}, invalidArgumentsError()
	}

	if err := validateReceiptPaths(arguments.Path); err != nil {
		return ToolResult{}, err
	}

	created, err := t.filesystem.Mkdir(arguments.Path)
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return NewToolResult(defaultVariant, createDirectoryResult{Path: arguments.Path, Created: created}), nil
}

type createDirectoryArguments struct {
	Path string `json:"path"`
}
type createDirectoryResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}
