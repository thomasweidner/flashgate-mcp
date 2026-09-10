package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const readFileToolName = "read_file"

// ReadFileTool exposes file reading as an MCP tool.
type ReadFileTool struct {
	filesystem     fs.FileSystem
	serverMaxBytes int64
}

// NewReadFileTool creates a new read_file tool.
func NewReadFileTool(filesystem fs.FileSystem, serverMaxBytes int64) *ReadFileTool {
	return &ReadFileTool{
		filesystem:     filesystem,
		serverMaxBytes: serverMaxBytes,
	}
}

// Name returns the tool name.
func (t *ReadFileTool) Name() string {
	return readFileToolName
}

// Title returns the human-readable tool title.
func (t *ReadFileTool) Title() string {
	return "Read File"
}

// Description returns the tool description.
func (t *ReadFileTool) Description() string {
	return "Reads a text file below the configured filesystem root."
}

// InputSchema returns the JSON schema for this tool.
func (t *ReadFileTool) InputSchema() any {
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative file path below the configured filesystem root.",
			},
			"maxBytes": map[string]any{
				"type":        "integer",
				"description": "Maximum number of bytes to read. Defaults to the configured maximum file size.",
				"minimum":     1,
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

// Definition returns the MCP tool definition.
func (t *ReadFileTool) Definition() protocol.Tool {
	return protocol.Tool{
		Name:         t.Name(),
		Title:        t.Title(),
		Description:  t.Description(),
		InputSchema:  t.InputSchema(),
		OutputSchema: filesystemOutputSchema(t.Name()),
	}
}

// Execute reads the requested file.
func (t *ReadFileTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments readFileArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	if !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}

	maxBytes := t.serverMaxBytes
	if arguments.MaxBytes != nil {
		if *arguments.MaxBytes < 1 {
			return nil, invalidParamsError()
		}
		maxBytes = *arguments.MaxBytes
	}
	if maxBytes > t.serverMaxBytes {
		maxBytes = t.serverMaxBytes
	}

	content, err := t.filesystem.Read(arguments.Path, maxBytes)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return readFileResult{
		Content: string(content),
		Size:    int64(len(content)),
	}, nil
}

type readFileArguments struct {
	Path     string `json:"path"`
	MaxBytes *int64 `json:"maxBytes,omitempty"`
}

type readFileResult struct {
	Content string `json:"content"`
	Size    int64  `json:"size"`
}
