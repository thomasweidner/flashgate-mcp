package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const readFileToolName = "read_file"

const (
	readModeText   = "text"
	readModeBinary = "binary"
	readModeAuto   = "auto"
)

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
	return "Reads bounded text, media, or binary content below the configured filesystem root."
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
			"mode": map[string]any{
				"type":        "string",
				"description": "Output mode. text (default) requires UTF-8 text, binary returns base64, and auto selects between them.",
				"enum":        []string{readModeText, readModeBinary, readModeAuto},
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

	mode := arguments.Mode
	if mode == "" {
		mode = readModeText
	}
	if mode != readModeText && mode != readModeBinary && mode != readModeAuto {
		return nil, invalidParamsError()
	}

	// Base64 expands every three raw bytes to four encoded bytes. Lower the
	// filesystem limit so encoded binary content cannot exceed the same
	// server-controlled byte ceiling used for inline text.
	readLimit := maxBytes
	if mode == readModeBinary || mode == readModeAuto {
		encodedSafeLimit := t.serverMaxBytes / 4 * 3
		if encodedSafeLimit < 1 {
			return nil, invalidParamsError()
		}
		if readLimit > encodedSafeLimit {
			readLimit = encodedSafeLimit
		}
	}

	content, err := t.filesystem.Read(arguments.Path, readLimit)
	if err != nil {
		return nil, mapFilesystemError(err)
	}

	mimeType := http.DetectContentType(content)
	isText := isUTF8Text(content)
	if mode == readModeAuto {
		if isText {
			mode = readModeText
		} else {
			mode = readModeBinary
		}
	}
	if mode == readModeText && !isText {
		return nil, &protocol.Error{Code: protocol.ErrInvalidParams, Message: "filesystem error: content is not UTF-8 text"}
	}

	encodedContent := string(content)
	encoding := "utf-8"
	if mode == readModeBinary {
		encodedContent = base64.StdEncoding.EncodeToString(content)
		encoding = "base64"
	}

	return readFileResult{
		Content:  encodedContent,
		Size:     int64(len(content)),
		MIMEType: mimeType,
		Encoding: encoding,
	}, nil
}

func isUTF8Text(content []byte) bool {
	return utf8.Valid(content) && bytes.IndexByte(content, 0) < 0
}

type readFileArguments struct {
	Path     string `json:"path"`
	MaxBytes *int64 `json:"maxBytes,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type readFileResult struct {
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	MIMEType string `json:"mimeType"`
	Encoding string `json:"encoding"`
}
