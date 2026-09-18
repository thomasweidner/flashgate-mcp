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
			"byteOffset": map[string]any{
				"type":        "integer",
				"description": "Zero-based byte offset. Use only with byteLength.",
				"minimum":     0,
			},
			"byteLength": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes returned from byteOffset.",
				"minimum":     0,
			},
			"headBytes": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes returned from the start of the file.",
				"minimum":     0,
			},
			"tailBytes": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes returned from the end of the file.",
				"minimum":     0,
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

	offset, length, ranged, valid := arguments.byteRange(maxBytes)
	if !valid {
		return nil, invalidParamsError()
	}
	var content []byte
	var err error
	if ranged {
		content, err = t.filesystem.ReadRange(arguments.Path, offset, length, maxBytes)
	} else {
		content, err = t.filesystem.Read(arguments.Path, maxBytes)
	}
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	return readFileResult{
		Content: string(content),
		Size:    int64(len(content)),
	}, nil
}

type readFileArguments struct {
	Path       string `json:"path"`
	MaxBytes   *int64 `json:"maxBytes,omitempty"`
	ByteOffset *int64 `json:"byteOffset,omitempty"`
	ByteLength *int64 `json:"byteLength,omitempty"`
	HeadBytes  *int64 `json:"headBytes,omitempty"`
	TailBytes  *int64 `json:"tailBytes,omitempty"`
}

func (a readFileArguments) byteRange(maxBytes int64) (offset, length int64, ranged, valid bool) {
	selectors := 0
	if a.ByteOffset != nil || a.ByteLength != nil {
		selectors++
		if a.ByteOffset == nil || a.ByteLength == nil || *a.ByteOffset < 0 || *a.ByteLength < 0 {
			return 0, 0, false, false
		}
		offset, length = *a.ByteOffset, *a.ByteLength
	}
	if a.HeadBytes != nil {
		selectors++
		if *a.HeadBytes < 0 {
			return 0, 0, false, false
		}
		offset, length = 0, *a.HeadBytes
	}
	if a.TailBytes != nil {
		selectors++
		if *a.TailBytes < 0 {
			return 0, 0, false, false
		}
		offset, length = -*a.TailBytes, *a.TailBytes
	}
	if selectors == 0 {
		return 0, 0, false, true
	}
	if selectors != 1 || length > maxBytes {
		return 0, 0, false, false
	}
	return offset, length, true, true
}

type readFileResult struct {
	Content string `json:"content"`
	Size    int64  `json:"size"`
}
