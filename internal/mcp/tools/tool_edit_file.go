package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const editFileToolName = "edit_file"

// EditFileTool performs one exact byte-range replacement in an existing file.
type EditFileTool struct{ filesystem fs.FileSystem }

func NewEditFileTool(filesystem fs.FileSystem) *EditFileTool {
	return &EditFileTool{filesystem: filesystem}
}
func (t *EditFileTool) Name() string  { return editFileToolName }
func (t *EditFileTool) Title() string { return "Edit File" }
func (t *EditFileTool) Description() string {
	return "Replaces an exact byte range or an expected number of exact text matches in an existing file below the configured filesystem root."
}
func (t *EditFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":            map[string]any{"type": "string", "minLength": 1, "description": "Relative file path below the configured filesystem root."},
			"startByte":       map[string]any{"type": "integer", "minimum": 0, "description": "Inclusive zero-based start byte."},
			"endByte":         map[string]any{"type": "integer", "minimum": 0, "description": "Exclusive zero-based end byte."},
			"content":         map[string]any{"type": "string", "description": "Replacement UTF-8 text; an empty string deletes the selected bytes."},
			"oldText":         map[string]any{"type": "string", "minLength": 1, "description": "Exact UTF-8 text to replace in match mode."},
			"newText":         map[string]any{"type": "string", "description": "Replacement UTF-8 text in match mode."},
			"expectedMatches": map[string]any{"type": "integer", "minimum": 1, "description": "Required exact non-overlapping match count; a different observed count rejects the edit without writing."},
		},
		"required": []string{"path"},
		"oneOf": []any{
			map[string]any{"required": []string{"startByte", "endByte", "content"}},
			map[string]any{"required": []string{"oldText", "newText", "expectedMatches"}},
		},
		"additionalProperties": false,
	}
}
func (t *EditFileTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}
func (t *EditFileTool) Execute(_ context.Context, raw json.RawMessage) (any, *protocol.Error) {
	var arguments editFileArguments
	if rpcErr := decodeStrictArguments(raw, &arguments); rpcErr != nil || !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}
	rangeMode := arguments.StartByte != nil || arguments.EndByte != nil || arguments.Content != nil
	matchMode := arguments.OldText != nil || arguments.NewText != nil || arguments.ExpectedMatches != nil
	if rangeMode == matchMode {
		return nil, invalidParamsError()
	}
	var size int64
	var err error
	if rangeMode {
		if arguments.StartByte == nil || arguments.EndByte == nil || arguments.Content == nil || *arguments.StartByte < 0 || *arguments.EndByte < *arguments.StartByte {
			return nil, invalidParamsError()
		}
		size, err = t.filesystem.EditRange(arguments.Path, *arguments.StartByte, *arguments.EndByte, []byte(*arguments.Content))
	} else {
		if arguments.OldText == nil || arguments.NewText == nil || arguments.ExpectedMatches == nil || *arguments.OldText == "" || *arguments.ExpectedMatches < 1 {
			return nil, invalidParamsError()
		}
		size, err = t.filesystem.EditMatches(arguments.Path, []byte(*arguments.OldText), []byte(*arguments.NewText), *arguments.ExpectedMatches)
	}
	if err != nil {
		if errors.Is(err, fs.ErrMatchCountMismatch) {
			return nil, invalidParamsError()
		}
		return nil, mapFilesystemError(err)
	}
	return editFileResult{Path: arguments.Path, Size: size, Edited: true}, nil
}

type editFileArguments struct {
	Path            string  `json:"path"`
	StartByte       *int64  `json:"startByte"`
	EndByte         *int64  `json:"endByte"`
	Content         *string `json:"content"`
	OldText         *string `json:"oldText"`
	NewText         *string `json:"newText"`
	ExpectedMatches *int    `json:"expectedMatches"`
}
type editFileResult struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Edited bool   `json:"edited"`
}
