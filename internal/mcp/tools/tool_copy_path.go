package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const copyPathToolName = "copy_path"

// CopyPathTool exposes filesystem copy operations as an MCP tool.
type CopyPathTool struct {
	filesystem fs.FileSystem
}

// NewCopyPathTool creates a new copy_path tool.
func NewCopyPathTool(filesystem fs.FileSystem) *CopyPathTool {
	return &CopyPathTool{
		filesystem: filesystem,
	}
}

// Name returns the tool name.
func (t *CopyPathTool) Name() string {
	return copyPathToolName
}

// Title returns the human-readable tool title.
func (t *CopyPathTool) Title() string {
	return "Copy Path"
}

// Description returns the tool description.
func (t *CopyPathTool) Description() string {
	return "Copies a file within one selected root. Directory copy is not supported."
}

// InputSchema returns the JSON schema for this tool.
func (t *CopyPathTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_root_id": rootIDSchema(),
			"target_root_id": rootIDSchema(),
			"source": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative source file path.",
			},
			"target": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative target file path.",
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
func (t *CopyPathTool) Definition() protocol.Tool {
	return protocol.Tool{
		Name:         t.Name(),
		Title:        t.Title(),
		Description:  t.Description(),
		InputSchema:  t.InputSchema(),
		OutputSchema: filesystemOutputSchema(t.Name()),
		Annotations:  filesystemToolAnnotations(t.Name()),
	}
}

func (t *CopyPathTool) ResultContract() ResultContract {
	return metadataContract(defaultVariant)
}

// Execute copies the requested source path to the target path.
func (t *CopyPathTool) Execute(ctx context.Context, rawArguments json.RawMessage) (ToolResult, error) {
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

	result, admit := mutationResult(ctx, t.filesystem, target.RootID, NewToolResult(defaultVariant, copyPathResult{Source: source.Path, Target: target.Path, Copied: true}), t.ResultContract())
	target.AdmitEffect = admit
	if err := t.filesystem.Copy(source, target, overwrite); err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return finishMutationResult(result)
}

type copyPathResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Copied bool   `json:"copied"`
}
