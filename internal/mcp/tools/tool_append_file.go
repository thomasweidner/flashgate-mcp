package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const appendFileToolName = "append_file"

// AppendFileTool exposes bounded file appends as an MCP tool.
type AppendFileTool struct {
	filesystem fs.FileSystem
}

// NewAppendFileTool creates a new append_file tool.
func NewAppendFileTool(filesystem fs.FileSystem) *AppendFileTool {
	return &AppendFileTool{filesystem: filesystem}
}

func (t *AppendFileTool) Name() string  { return appendFileToolName }
func (t *AppendFileTool) Title() string { return "Append File" }
func (t *AppendFileTool) Description() string {
	return "Appends bounded text content to a file below the configured filesystem root, creating the file when absent."
}

func (t *AppendFileTool) InputSchema() any {
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
				"description": "Text content to append. An empty string is allowed.",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (t *AppendFileTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

func (t *AppendFileTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments appendFileArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}
	if !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}
	content := []byte(arguments.Content)
	if err := t.filesystem.Append(arguments.Path, content); err != nil {
		return nil, mapFilesystemError(err)
	}
	return appendFileResult{Path: arguments.Path, Size: int64(len(content)), Appended: true}, nil
}

type appendFileArguments struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type appendFileResult struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Appended bool   `json:"appended"`
}
