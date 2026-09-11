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
			"path":   map[string]any{"type": "string", "minLength": 1, "description": "Relative directory path below the configured filesystem root."},
			"dryRun": dryRunInputSchema(),
		},
		"required": []string{"path"}, "additionalProperties": false,
	}
}
func (t *CreateDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}
func (t *CreateDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments createDirectoryArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}
	if arguments.DryRun {
		if err := t.filesystem.ValidatePath(arguments.Path, false); err != nil {
			return nil, mapFilesystemError(err)
		}
		return createDirectoryResult{Path: arguments.Path, DryRun: true}, nil
	}

	created, err := t.filesystem.Mkdir(arguments.Path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return createDirectoryResult{Path: arguments.Path, Created: created}, nil
}

type createDirectoryArguments struct {
	Path   string `json:"path"`
	DryRun bool   `json:"dryRun,omitempty"`
}
type createDirectoryResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
	DryRun  bool   `json:"dryRun,omitempty"`
}
