package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/search"
)

const (
	searchPathsToolName  = "search_paths"
	maxSearchPathResults = 1000
)

// SearchPathsTool exposes bounded recursive path search as an MCP tool.
type SearchPathsTool struct {
	service *search.PathService
}

// NewSearchPathsTool creates a search_paths tool over the central filesystem
// abstraction.
func NewSearchPathsTool(filesystem fs.DirectoryLister) *SearchPathsTool {
	service, err := search.NewPathService(filesystem, maxSearchPathResults)
	if err != nil {
		panic(err)
	}
	return &SearchPathsTool{service: service}
}

func (t *SearchPathsTool) Name() string  { return searchPathsToolName }
func (t *SearchPathsTool) Title() string { return "Search Paths" }
func (t *SearchPathsTool) Description() string {
	return "Recursively lists root-relative paths below the configured filesystem root."
}
func (t *SearchPathsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative directory path at which to start. Defaults to '.' when omitted.",
			},
		},
		"additionalProperties": false,
	}
}
func (t *SearchPathsTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

func (t *SearchPathsTool) Execute(ctx context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments searchPathsArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	startPath := "."
	if arguments.Path != nil {
		if !isNonBlank(*arguments.Path) {
			return nil, invalidParamsError()
		}
		startPath = *arguments.Path
	}

	paths, err := t.service.Search(ctx, startPath)
	if err != nil {
		switch {
		case errors.Is(err, search.ErrLimitExceeded):
			return nil, &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: limit exceeded"}
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "search error: canceled"}
		default:
			return nil, mapFilesystemError(err)
		}
	}

	return searchPathsResult{Paths: paths}, nil
}

type searchPathsArguments struct {
	Path *string `json:"path,omitempty"`
}

type searchPathsResult struct {
	Paths []search.Path `json:"paths"`
}
