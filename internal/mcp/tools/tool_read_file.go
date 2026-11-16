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
	return "Reads a UTF-8 text file below the selected root."
}

// InputSchema returns the JSON schema for this tool.
func (t *ReadFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root_id": rootIDSchema(),
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative file path.",
			},
			"maxBytes": map[string]any{
				"type":        "integer",
				"description": "Maximum number of bytes to read. Defaults to the configured maximum file size.",
				"minimum":     1,
			},
		},
		"required":             []string{"root_id", "path"},
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
	var path fs.Operand
	var requestedMaxBytes *int64
	if err := decodeStrictArguments(rawArguments, map[string]any{
		"root_id":  &path.RootID,
		"path":     &path.Path,
		"maxBytes": &requestedMaxBytes,
	}); err != nil {
		return ToolResult{}, err
	}
	if !validOperand(path) {
		return ToolResult{}, invalidArgumentsError()
	}
	maxBytes := t.serverMaxBytes
	if requestedMaxBytes != nil {
		if *requestedMaxBytes < 1 {
			return ToolResult{}, invalidArgumentsError()
		}
		maxBytes = *requestedMaxBytes
	}
	if maxBytes > t.serverMaxBytes {
		maxBytes = t.serverMaxBytes
	}

	content, err := t.filesystem.Read(path, maxBytes)
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
