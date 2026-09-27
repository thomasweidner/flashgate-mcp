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
	return "Lists files and directories below the configured filesystem root."
}
func (t *ListDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative directory path below the configured filesystem root. Defaults to '.' when omitted.",
			},
		},
		"additionalProperties": false,
	}
}
func (t *ListDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name()), Annotations: filesystemToolAnnotations(t.Name())}
}

func (t *ListDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, error) {
	var arguments listDirectoryArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	path := "."
	if arguments.Path != nil {
		if !isNonBlank(*arguments.Path) {
			return nil, invalidArgumentsError()
		}
		path = *arguments.Path
	}

	entries, err := t.filesystem.List(path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return listDirectoryResult{Entries: entries}, nil
}

type listDirectoryArguments struct {
	Path *string `json:"path,omitempty"`
}

type listDirectoryResult struct {
	Entries []fs.Entry `json:"entries"`
}
