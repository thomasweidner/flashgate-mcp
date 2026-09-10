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
	return "Copies a file below the configured filesystem root. Directory copy is not supported."
}

// InputSchema returns the JSON schema for this tool.
func (t *CopyPathTool) InputSchema() any {
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
		"properties": map[string]any{
			"source": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative source file path below the configured filesystem root.",
			},
			"target": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative target file path below the configured filesystem root.",
			},
			"overwrite": map[string]any{
				"type":        "boolean",
				"description": "Whether an existing target may be overwritten. Defaults to false.",
			},
		},
		"required":             []string{"source", "target"},
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
	}
}

// Execute copies the requested source path to the target path.
func (t *CopyPathTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments copyPathArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	if !isNonBlank(arguments.Source) || !isNonBlank(arguments.Target) {
		return nil, invalidParamsError()
	}

	if err := t.filesystem.Copy(arguments.Source, arguments.Target, arguments.Overwrite); err != nil {
		return nil, mapFilesystemError(err)
	}

	return copyPathResult{
		Source: arguments.Source,
		Target: arguments.Target,
		Copied: true,
	}, nil
}

type copyPathArguments struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

type copyPathResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Copied bool   `json:"copied"`
}
