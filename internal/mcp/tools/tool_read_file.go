package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/wiretrace"
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
	return "Reads a UTF-8 text file below the configured filesystem root."
}

// InputSchema returns the JSON schema for this tool.
func (t *ReadFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
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
		Annotations:  filesystemToolAnnotations(t.Name()),
	}
}

func (t *ReadFileTool) ResultContract() ResultContract {
	return ResultContract{{
		Name: defaultVariant, ObjectKind: MetadataPayload, Parts: []PayloadKind{TextPayload},
		Mode: InlineTransfer, State: CompleteResult, MaxObjectBytes: 128, MaxPayloadBytes: t.serverMaxBytes,
	}}
}

// Execute reads the requested file.
func (t *ReadFileTool) Execute(ctx context.Context, rawArguments json.RawMessage) (ToolResult, error) {
	var arguments readFileArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return ToolResult{}, rpcErr
	}

	if !isNonBlank(arguments.Path) {
		return ToolResult{}, invalidArgumentsError()
	}

	maxBytes := t.serverMaxBytes
	if arguments.MaxBytes != nil {
		if *arguments.MaxBytes < 1 {
			return ToolResult{}, invalidArgumentsError()
		}
		maxBytes = *arguments.MaxBytes
	}
	if maxBytes > t.serverMaxBytes {
		maxBytes = t.serverMaxBytes
	}

	content, err := t.filesystem.Read(arguments.Path, maxBytes)
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}

	size := int64(len(content))
	if size > maxBytes {
		return ToolResult{}, mapFilesystemError(fs.ErrFileTooLarge)
	}
	text, err := TextPart(wiretrace.Text(ctx, content))
	if err != nil {
		return ToolResult{}, &toolExecutionError{
			category: categoryInvalidTextEncoding,
			message:  "file text is not valid UTF-8",
		}
	}
	return NewPayloadResult(defaultVariant, text), nil
}

type readFileArguments struct {
	Path     string `json:"path"`
	MaxBytes *int64 `json:"maxBytes,omitempty"`
}
