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
	return "Deletes a file or directory below the configured filesystem root."
}

// InputSchema returns the JSON schema for this tool.
func (t *DeletePathTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative file or directory path below the configured filesystem root.",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "Whether a non-empty directory may be deleted recursively. Defaults to false.",
			},
		},
		"required":             []string{"path"},
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
		Annotations:  destructiveWriteAnnotations,
	}
}

// Execute deletes the requested path.
func (t *DeletePathTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments deletePathArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	if !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}

	if err := t.filesystem.Delete(arguments.Path, arguments.Recursive); err != nil {
		return nil, mapFilesystemError(err)
	}

	return deletePathResult{
		Path:    arguments.Path,
		Deleted: true,
	}, nil
}

type deletePathArguments struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type deletePathResult struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}
