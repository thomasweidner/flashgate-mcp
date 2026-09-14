package tools

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/search"
)

const (
	searchPathsToolName     = "search_paths"
	maxSearchPathResults    = 1000
	maxSearchContentFiles   = 1000
	maxSearchBytesPerFile   = 1024 * 1024
	maxSearchScannedBytes   = 10 * 1024 * 1024
	maxSearchMatchesPerFile = 256
	maxSearchContentMatches = 1000
	maxSearchResponseBytes  = 1024 * 1024
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
	return "Recursively searches root-relative paths or bounded file content with optional filename and portable metadata filters."
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
			"text": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Case-sensitive UTF-8 literal text to find in files.",
			},
			"regex": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Go RE2-style regular expression to find in files.",
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
		"allOf": []any{
			map[string]any{"not": map[string]any{"required": []string{"name", "namePattern"}}},
			map[string]any{"not": map[string]any{"required": []string{"text", "regex"}}},
		},
		"additionalProperties": false,
	}
}
func (t *SearchPathsTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

func (t *SearchPathsTool) Execute(ctx context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	if !utf8.Valid(rawArguments) {
		return nil, invalidParamsError()
	}
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
	if arguments.Text != nil && arguments.Regex != nil {
		return nil, invalidParamsError()
	}
	if arguments.Text != nil && *arguments.Text == "" || arguments.Regex != nil && *arguments.Regex == "" {
		return nil, invalidParamsError()
	}
	if (arguments.Text != nil || arguments.Regex != nil) && arguments.Type != nil && *arguments.Type == "directory" {
		return nil, invalidParamsError()
	}
	if arguments.Text != nil {
		matches, err := t.service.SearchLiteral(ctx, startPath, selector, kind, filter, *arguments.Text, contentSearchLimits())
		if err != nil {
			return nil, mapSearchError(err)
		}
		return searchContentResult{Matches: matches}, nil
	}
	if arguments.Regex != nil {
		matches, err := t.service.SearchRegex(ctx, startPath, selector, kind, filter, *arguments.Regex, contentSearchLimits())
		if err != nil {
			return nil, mapSearchError(err)
		}
		return searchContentResult{Matches: matches}, nil
	}
	paths, err := t.service.SearchFiltered(ctx, startPath, selector, kind, filter)
	if err != nil {
		return nil, mapSearchError(err)
	}

	return searchPathsResult{Paths: paths}, nil
}

type searchPathsArguments struct {
	Path              *string `json:"path,omitempty"`
	Name              *string `json:"name,omitempty"`
	NamePattern       *string `json:"namePattern,omitempty"`
	Text              *string `json:"text,omitempty"`
	Regex             *string `json:"regex,omitempty"`
	Type              *string `json:"type,omitempty"`
	MinSizeBytes      *int64  `json:"minSizeBytes,omitempty"`
	MaxSizeBytes      *int64  `json:"maxSizeBytes,omitempty"`
	ModifiedNotBefore *string `json:"modifiedNotBefore,omitempty"`
	ModifiedNotAfter  *string `json:"modifiedNotAfter,omitempty"`
}

func contentSearchLimits() search.LiteralLimits {
	return search.LiteralLimits{
		MaxFiles: maxSearchContentFiles, MaxBytesPerFile: maxSearchBytesPerFile,
		MaxScannedBytes: maxSearchScannedBytes, MaxMatchesPerFile: maxSearchMatchesPerFile,
		MaxMatches: maxSearchContentMatches, MaxResponseBytes: maxSearchResponseBytes,
	}
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

type searchContentResult struct {
	Matches []search.LiteralMatch `json:"matches"`
}

func mapSearchError(err error) *protocol.Error {
	switch {
	case errors.Is(err, search.ErrInvalidNameSelector), errors.Is(err, search.ErrInvalidMetadataFilter), errors.Is(err, search.ErrInvalidLiteralSearch), errors.Is(err, search.ErrInvalidRegexSearch):
		return invalidParamsError()
	case errors.Is(err, search.ErrLimitExceeded), errors.Is(err, search.ErrScanLimitExceeded), errors.Is(err, search.ErrMatchLimitExceeded), errors.Is(err, search.ErrResponseLimitExceeded):
		return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: limit exceeded", Category: "limit_exceeded"}
	case errors.Is(err, fs.ErrFileTooLarge):
		return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: limit exceeded", Category: "limit_exceeded"}
	case errors.Is(err, search.ErrContentSearchUnavailable):
		return &protocol.Error{Code: protocol.ErrInternalError, Message: "search error: unavailable", Category: "unavailable"}
	case errors.Is(err, context.Canceled):
		return &protocol.Error{Code: protocol.ErrInternalError, Message: "search error: canceled", Category: "canceled"}
	case errors.Is(err, context.DeadlineExceeded):
		return &protocol.Error{Code: protocol.ErrInternalError, Message: "search error: deadline exceeded", Category: "deadline_exceeded"}
	default:
		return mapFilesystemError(err)
	}
}
