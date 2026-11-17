package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const writeFileToolName = "write_file"

// WriteFileTool exposes file writing as an MCP tool.
type WriteFileTool struct {
	filesystem fs.FileSystem
}

// NewWriteFileTool creates a new write_file tool.
func NewWriteFileTool(filesystem fs.FileSystem) *WriteFileTool {
	return &WriteFileTool{
		filesystem: filesystem,
	}
}

// Name returns the tool name.
func (t *WriteFileTool) Name() string {
	return writeFileToolName
}

// Title returns the human-readable tool title.
func (t *WriteFileTool) Title() string {
	return "Write File"
}

// Description returns the tool description.
func (t *WriteFileTool) Description() string {
	return "Writes a text file below the selected root."
}

// InputSchema returns the JSON schema for this tool.
func (t *WriteFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root_id": rootIDSchema(),
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative file path.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Text content to write. An empty string is allowed.",
			},
			"overwrite": map[string]any{
				"type":        "boolean",
				"description": "Whether an existing file may be overwritten. Defaults to false.",
			},
		},
		"required":             []string{"root_id", "path"},
		"additionalProperties": false,
	}
}

// Definition returns the MCP tool definition.
func (t *WriteFileTool) Definition() protocol.Tool {
	return protocol.Tool{
		Name:         t.Name(),
		Title:        t.Title(),
		Description:  t.Description(),
		InputSchema:  t.InputSchema(),
		OutputSchema: filesystemOutputSchema(t.Name()),
		Annotations:  filesystemToolAnnotations(t.Name()),
	}
}

func (t *WriteFileTool) ResultContract() ResultContract {
	return metadataContract(defaultVariant)
}

// Execute writes the requested file.
func (t *WriteFileTool) Execute(ctx context.Context, rawArguments json.RawMessage) (ToolResult, error) {
	var path fs.Operand
	var text string
	var overwrite bool
	if err := decodeStrictArguments(rawArguments, map[string]any{
		"root_id":   &path.RootID,
		"path":      &path.Path,
		"content":   &text,
		"overwrite": &overwrite,
	}); err != nil {
		return ToolResult{}, err
	}
	if !validOperand(path) {
		return ToolResult{}, invalidArgumentsError()
	}
	if err := validateReceiptPaths(path.Path); err != nil {
		return ToolResult{}, err
	}

	content := []byte(text)
	result, admit := mutationResult(ctx, t.filesystem, path.RootID, NewToolResult(defaultVariant, writeFileResult{
		Path: path.Path, Size: int64(len(content)), Written: true,
	}), t.ResultContract())
	path.AdmitEffect = admit

	if err := t.filesystem.Write(path, content, overwrite); err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	return finishMutationResult(result)
}

type writeFileResult struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Written bool   `json:"written"`
}
