package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
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

func (t *CreateDirectoryTool) Execute(ctx context.Context, rawArguments json.RawMessage) (ToolResult, error) {
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

	// Admit both outcomes without serializing a speculative receipt. Around
	// the metadata-parity threshold, the shorter object can have a larger wrapper.
	admitted := false
	path.AdmitEffect = func() error {
		if admitted {
			return nil
		}
		limits, err := t.filesystem.LimitsForRoot(path.RootID)
		if err != nil {
			return err
		}
		needed := max(mkdirReceiptSize(path.Path, true), mkdirReceiptSize(path.Path, false))
		if needed > resource.ResultLimit(ctx, limits.Result) {
			return fs.ErrLimitExceeded
		}
		admitted = true
		return nil
	}
	created, err := t.filesystem.Mkdir(path)
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	if !admitted {
		return ToolResult{}, errInvalidToolResult
	}
	return prepareResult(ctx, t.filesystem, path.RootID, NewToolResult(defaultVariant, createDirectoryResult{Path: path.Path, Created: created}), t.ResultContract())
}

type createDirectoryResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}
