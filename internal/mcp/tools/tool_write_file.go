package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const writeFileToolName = "write_file"

var sha256Pattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

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
			"expectedSha256": map[string]any{
				"type": "string", "pattern": "^[0-9a-fA-F]{64}$",
				"description": "Require the existing file to have this SHA-256 digest.",
			},
			"expectedModifiedTime": map[string]any{
				"type": "string", "format": "date-time",
				"description": "Require the existing file to have this RFC 3339 modification time.",
			},
			"expectedPathType": map[string]any{
				"type": "string", "enum": []string{"missing", "file"},
				"description": "Require the target to be missing or a regular file.",
			},
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

	preconditions, rpcErr := arguments.preconditions()
	if rpcErr != nil {
		return nil, rpcErr
	}
	if err := writeWithPreconditions(t.filesystem, arguments.Path, content, arguments.Overwrite, preconditions); err != nil {
		return nil, mapFilesystemError(err)
	}

	return writeFileResult{
		Path:    arguments.Path,
		Size:    int64(len(content)),
		Written: true,
	}, nil
}

func writeWithPreconditions(filesystem fs.FileSystem, path string, content []byte, overwrite bool, preconditions fs.WritePreconditions) error {
	if preconditions.SHA256 == nil && preconditions.ModifiedTime == nil && preconditions.PathType == nil {
		return filesystem.Write(path, content, overwrite)
	}
	return filesystem.WriteConditional(path, content, overwrite, preconditions)
}

type writeFileArguments struct {
	Path                 string  `json:"path"`
	Content              string  `json:"content,omitempty"`
	Overwrite            bool    `json:"overwrite,omitempty"`
	ExpectedSHA256       *string `json:"expectedSha256,omitempty"`
	ExpectedModifiedTime *string `json:"expectedModifiedTime,omitempty"`
	ExpectedPathType     *string `json:"expectedPathType,omitempty"`
}

func (a writeFileArguments) preconditions() (fs.WritePreconditions, *protocol.Error) {
	result := fs.WritePreconditions{SHA256: a.ExpectedSHA256, PathType: a.ExpectedPathType}
	if a.ExpectedSHA256 != nil && !sha256Pattern.MatchString(*a.ExpectedSHA256) {
		return fs.WritePreconditions{}, invalidParamsError()
	}
	if a.ExpectedPathType != nil && *a.ExpectedPathType != "missing" && *a.ExpectedPathType != "file" {
		return fs.WritePreconditions{}, invalidParamsError()
	}
	if a.ExpectedModifiedTime != nil {
		parsed, err := time.Parse(time.RFC3339Nano, *a.ExpectedModifiedTime)
		if err != nil {
			return fs.WritePreconditions{}, invalidParamsError()
		}
		result.ModifiedTime = &parsed
	}
	return result, nil
}

type writeFileResult struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Written bool   `json:"written"`
}
