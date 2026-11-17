package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const movePathToolName = "move_path"

// MovePathTool exposes filesystem move operations as an MCP tool.
type MovePathTool struct {
	filesystem fs.FileSystem
}

// NewMovePathTool creates a new move_path tool.
func NewMovePathTool(filesystem fs.FileSystem) *MovePathTool {
	return &MovePathTool{
		filesystem: filesystem,
	}
}

// Name returns the tool name.
func (t *MovePathTool) Name() string {
	return movePathToolName
}

// Title returns the human-readable tool title.
func (t *MovePathTool) Title() string {
	return "Move Path"
}

// Description returns the tool description.
func (t *MovePathTool) Description() string {
	return "Moves or renames a file or directory on the same volume within one selected root."
}

// InputSchema returns the JSON schema for this tool.
func (t *MovePathTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_root_id": rootIDSchema(),
			"target_root_id": rootIDSchema(),
			"source": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative source file or directory path.",
			},
			"target": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative target file or directory path.",
			},
			"overwrite": map[string]any{
				"type":        "boolean",
				"description": "Whether an existing target may be overwritten. Defaults to false.",
			},
		},
		"required":             []string{"source_root_id", "source", "target_root_id", "target"},
		"additionalProperties": false,
	}
}

// Definition returns the MCP tool definition.
func (t *MovePathTool) Definition() protocol.Tool {
	return protocol.Tool{
		Name:         t.Name(),
		Title:        t.Title(),
		Description:  t.Description(),
		InputSchema:  t.InputSchema(),
		OutputSchema: filesystemOutputSchema(t.Name()),
		Annotations:  filesystemToolAnnotations(t.Name()),
	}
}

func (t *MovePathTool) ResultContract() ResultContract {
	return metadataContract(defaultVariant)
}

// Execute moves the requested source path to the target path.
func (t *MovePathTool) Execute(ctx context.Context, rawArguments json.RawMessage) (ToolResult, error) {
	var source, target fs.Operand
	var overwrite bool
	if err := decodeStrictArguments(rawArguments, map[string]any{
		"source_root_id": &source.RootID,
		"source":         &source.Path,
		"target_root_id": &target.RootID,
		"target":         &target.Path,
		"overwrite":      &overwrite,
	}); err != nil {
		return ToolResult{}, err
	}
	if !validOperand(source) || !validOperand(target) {
		return ToolResult{}, invalidArgumentsError()
	}
	if err := validateReceiptPaths(source.Path, target.Path); err != nil {
		return ToolResult{}, err
	}

	if err := validatePairLimits(t.filesystem, source, target); err != nil {
		return ToolResult{}, err
	}
	result, admit := mutationResult(ctx, t.filesystem, target.RootID, NewToolResult(defaultVariant, movePathResult{Source: source.Path, Target: target.Path, Moved: true}), t.ResultContract())
	target.AdmitEffect = admit
	if err := t.filesystem.Move(source, target, overwrite); err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return finishMutationResult(result)
}

type movePathResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Moved  bool   `json:"moved"`
}
