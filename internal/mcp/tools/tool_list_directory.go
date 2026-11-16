package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
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
	return "Lists files and directories below the selected root."
}
func (t *ListDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root_id": rootIDSchema(),
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative directory path. Use '.' for the root directory.",
			},
		},
		"required":             []string{"root_id", "path"},
		"additionalProperties": false,
	}
}
func (t *ListDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name()), Annotations: filesystemToolAnnotations(t.Name())}
}

func (t *ListDirectoryTool) ResultContract() ResultContract {
	return ResultContract{{Name: defaultVariant, ObjectKind: StructuredPayload, Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: maxStructuredBytes}}
}

func (t *ListDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (ToolResult, error) {
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

	entries, err := t.filesystem.List(path)
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return NewToolResult(defaultVariant, listDirectoryResult{Entries: entries}), nil
}

type listDirectoryResult struct {
	Entries []fs.Entry `json:"entries"`
}
