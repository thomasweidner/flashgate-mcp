package tools

import (
	"context"
	"encoding/json"
	"errors"
	"time"

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
	return "Recursively searches root-relative paths with optional filename and portable metadata filters."
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
			"name": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Exact case-sensitive base filename to match.",
			},
			"namePattern": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Case-sensitive path.Match pattern applied to each complete base filename.",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"file", "directory"},
				"description": "Portable entry type to return.",
			},
			"minSizeBytes":      map[string]any{"type": "integer", "minimum": 0, "description": "Inclusive minimum file size in bytes; directories never match size filters."},
			"maxSizeBytes":      map[string]any{"type": "integer", "minimum": 0, "description": "Inclusive maximum file size in bytes; directories never match size filters."},
			"modifiedNotBefore": map[string]any{"type": "string", "format": "date-time", "description": "Inclusive RFC 3339 lower modification-time bound."},
			"modifiedNotAfter":  map[string]any{"type": "string", "format": "date-time", "description": "Inclusive RFC 3339 upper modification-time bound."},
		},
		"not":                  map[string]any{"required": []string{"name", "namePattern"}},
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

	filter, rpcErr := arguments.metadataFilter()
	if rpcErr != nil {
		return nil, rpcErr
	}
	selector := ""
	kind := search.NameMatchLiteral
	switch {
	case arguments.Name != nil && arguments.NamePattern != nil:
		return nil, invalidParamsError()
	case arguments.Name != nil:
		if !isNonBlank(*arguments.Name) {
			return nil, invalidParamsError()
		}
		selector = *arguments.Name
	case arguments.NamePattern != nil:
		if !isNonBlank(*arguments.NamePattern) {
			return nil, invalidParamsError()
		}
		selector, kind = *arguments.NamePattern, search.NameMatchPattern
	}
	paths, err := t.service.SearchFiltered(ctx, startPath, selector, kind, filter)
	if err != nil {
		switch {
		case errors.Is(err, search.ErrInvalidNameSelector), errors.Is(err, search.ErrInvalidMetadataFilter):
			return nil, invalidParamsError()
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
	Path              *string `json:"path,omitempty"`
	Name              *string `json:"name,omitempty"`
	NamePattern       *string `json:"namePattern,omitempty"`
	Type              *string `json:"type,omitempty"`
	MinSizeBytes      *int64  `json:"minSizeBytes,omitempty"`
	MaxSizeBytes      *int64  `json:"maxSizeBytes,omitempty"`
	ModifiedNotBefore *string `json:"modifiedNotBefore,omitempty"`
	ModifiedNotAfter  *string `json:"modifiedNotAfter,omitempty"`
}

func (a searchPathsArguments) metadataFilter() (search.MetadataFilter, *protocol.Error) {
	filter := search.MetadataFilter{Type: search.EntryTypeAny, MinSizeBytes: a.MinSizeBytes, MaxSizeBytes: a.MaxSizeBytes}
	if a.Type != nil {
		switch *a.Type {
		case "file":
			filter.Type = search.EntryTypeFile
		case "directory":
			filter.Type = search.EntryTypeDirectory
		default:
			return search.MetadataFilter{}, invalidParamsError()
		}
	}
	parseTime := func(value *string) (*time.Time, bool) {
		if value == nil {
			return nil, true
		}
		parsed, err := time.Parse(time.RFC3339, *value)
		if err != nil {
			return nil, false
		}
		return &parsed, true
	}
	var valid bool
	if filter.ModifiedNotBefore, valid = parseTime(a.ModifiedNotBefore); !valid {
		return search.MetadataFilter{}, invalidParamsError()
	}
	if filter.ModifiedNotAfter, valid = parseTime(a.ModifiedNotAfter); !valid {
		return search.MetadataFilter{}, invalidParamsError()
	}
	if filter.MinSizeBytes != nil && *filter.MinSizeBytes < 0 || filter.MaxSizeBytes != nil && *filter.MaxSizeBytes < 0 ||
		filter.MinSizeBytes != nil && filter.MaxSizeBytes != nil && *filter.MinSizeBytes > *filter.MaxSizeBytes ||
		filter.ModifiedNotBefore != nil && filter.ModifiedNotAfter != nil && filter.ModifiedNotBefore.After(*filter.ModifiedNotAfter) {
		return search.MetadataFilter{}, invalidParamsError()
	}
	return filter, nil
}

type searchPathsResult struct {
	Paths []search.Path `json:"paths"`
}
