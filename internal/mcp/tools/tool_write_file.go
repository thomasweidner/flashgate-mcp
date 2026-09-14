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
	return "Writes a text file below the configured filesystem root."
}

// InputSchema returns the JSON schema for this tool.
func (t *WriteFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative file path below the configured filesystem root.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Text content to write. An empty string is allowed.",
			},
			"overwrite": map[string]any{
				"type":        "boolean",
				"description": "Whether an existing file may be overwritten. Defaults to false.",
			},
			"dryRun": dryRunInputSchema(),
		},
		"required":             []string{"path"},
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
	}
}

// Execute writes the requested file.
func (t *WriteFileTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments writeFileArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	if !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}

	content := []byte(arguments.Content)
	if arguments.DryRun {
		if err := t.filesystem.ValidatePath(arguments.Path, false); err != nil {
			return nil, mapFilesystemError(err)
		}
		return writeFileResult{Path: arguments.Path, Size: int64(len(content)), DryRun: true}, nil
	}

	if err := t.filesystem.Write(arguments.Path, content, arguments.Overwrite); err != nil {
		return nil, mapFilesystemError(err)
	}

	return writeFileResult{
		Path:    arguments.Path,
		Size:    int64(len(content)),
		Written: true,
	}, nil
}

type writeFileArguments struct {
	Path      string `json:"path"`
	Content   string `json:"content,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
	DryRun    bool   `json:"dryRun,omitempty"`
}

type writeFileResult struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Written bool   `json:"written"`
	DryRun  bool   `json:"dryRun,omitempty"`
}
