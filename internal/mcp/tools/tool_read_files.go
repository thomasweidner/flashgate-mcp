package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const (
	readFilesToolName = "read_files"
	maxReadFilesPaths = 100
)

// ReadFilesTool exposes bounded batch file reads as an MCP tool.
type ReadFilesTool struct {
	filesystem     fs.FileSystem
	serverMaxBytes int64
}

// NewReadFilesTool creates a new read_files tool.
func NewReadFilesTool(filesystem fs.FileSystem, serverMaxBytes int64) *ReadFilesTool {
	return &ReadFilesTool{filesystem: filesystem, serverMaxBytes: serverMaxBytes}
}

func (t *ReadFilesTool) Name() string  { return readFilesToolName }
func (t *ReadFilesTool) Title() string { return "Read Files" }
func (t *ReadFilesTool) Description() string {
	return "Reads a bounded batch of text files below the configured filesystem root."
}

func (t *ReadFilesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paths": map[string]any{
				"type": "array", "minItems": 1, "maxItems": maxReadFilesPaths,
				"items":       map[string]any{"type": "string", "minLength": 1},
				"description": "Ordered relative file paths below the configured filesystem root.",
			},
			"maxBytesPerFile": map[string]any{
				"type": "integer", "minimum": 1,
				"description": "Maximum bytes per file. Defaults to and cannot exceed the configured maximum file size.",
			},
		},
		"required": []string{"paths"}, "additionalProperties": false,
	}
}

func (t *ReadFilesTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

func (t *ReadFilesTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments readFilesArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}
	if len(arguments.Paths) == 0 || len(arguments.Paths) > maxReadFilesPaths {
		return nil, invalidParamsError()
	}
	for _, path := range arguments.Paths {
		if !isNonBlank(path) {
			return nil, invalidParamsError()
		}
	}

	perFileLimit := t.serverMaxBytes
	if arguments.MaxBytesPerFile != nil {
		if *arguments.MaxBytesPerFile < 1 {
			return nil, invalidParamsError()
		}
		perFileLimit = min(*arguments.MaxBytesPerFile, t.serverMaxBytes)
	}

	result := readFilesResult{Results: make([]readFilesItemResult, 0, len(arguments.Paths)), Accepted: len(arguments.Paths)}
	var totalBytes int64
	for _, path := range arguments.Paths {
		remaining := t.serverMaxBytes - totalBytes
		if remaining <= 0 {
			result.Results = append(result.Results, failedReadFilesItem(path, "aggregate_limit_exceeded"))
			result.Failed++
			continue
		}
		content, err := t.filesystem.Read(path, min(perFileLimit, remaining))
		if err != nil {
			result.Results = append(result.Results, failedReadFilesItem(path, readFilesErrorCategory(err)))
			result.Failed++
			continue
		}
		size := int64(len(content))
		text := string(content)
		result.Results = append(result.Results, readFilesItemResult{Path: path, Content: &text, Size: &size})
		result.Completed++
		totalBytes += size
	}
	result.TotalBytes = totalBytes
	return result, nil
}

type readFilesArguments struct {
	Paths           []string `json:"paths"`
	MaxBytesPerFile *int64   `json:"maxBytesPerFile,omitempty"`
}

type readFilesResult struct {
	Results    []readFilesItemResult `json:"results"`
	Accepted   int                   `json:"accepted"`
	Completed  int                   `json:"completed"`
	Failed     int                   `json:"failed"`
	TotalBytes int64                 `json:"totalBytes"`
}

type readFilesItemResult struct {
	Path    string  `json:"path"`
	Content *string `json:"content,omitempty"`
	Size    *int64  `json:"size,omitempty"`
	Error   string  `json:"error,omitempty"`
}

func failedReadFilesItem(path, category string) readFilesItemResult {
	return readFilesItemResult{Path: path, Error: category}
}

func readFilesErrorCategory(err error) string {
	switch {
	case errors.Is(err, fs.ErrFileTooLarge), errors.Is(err, fs.ErrLimitExceeded):
		return "limit_exceeded"
	case errors.Is(err, fs.ErrPathIsDirectory):
		return "path_is_directory"
	case errors.Is(err, fs.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return "not_found"
	default:
		return "read_failed"
	}
}
