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
	searchPathsToolName           = "search_paths"
	maxSearchPathResults          = 1000
	maxSearchDepth                = 64
	maxSearchVisitedEntries       = 10000
	maxSearchContentFiles         = 1000
	maxSearchBytesPerFile         = 1024 * 1024
	maxSearchScannedBytes         = 10 * 1024 * 1024
	maxSearchMatchesPerFile       = 256
	maxSearchContentMatches       = 1000
	maxSearchResponseBytes        = 1024 * 1024
	maxSearchContextLines         = 10
	maxSearchContextBytesPerMatch = 16 * 1024
	maxSearchContextBytes         = 256 * 1024
)

// SearchPathsTool exposes bounded recursive path search as an MCP tool.
type SearchPathsTool struct {
	service *search.PathService
}

// NewSearchPathsTool creates a search_paths tool over the central filesystem
// abstraction.
func NewSearchPathsTool(filesystem fs.DirectoryLister) *SearchPathsTool {
	service, err := search.NewPathService(filesystem, maxSearchPathResults, maxSearchDepth, maxSearchVisitedEntries)
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
			"maxMatchesPerFile": map[string]any{"type": "integer", "minimum": 1, "maximum": maxSearchMatchesPerFile, "description": "Optional per-file match limit, capped by the server maximum."},
			"maxMatches":        map[string]any{"type": "integer", "minimum": 1, "maximum": maxSearchContentMatches, "description": "Optional total match limit, capped by the server maximum."},
			"contextLines":      map[string]any{"type": "integer", "minimum": 1, "maximum": maxSearchContextLines, "description": "Optional number of complete UTF-8 lines before and after each content match."},
			"binaryMode":        map[string]any{"type": "string", "enum": []string{"skip", "error", "explicit"}, "description": "Binary/non-UTF-8 handling: skip with diagnostics (default), fail, or explicitly byte-search without context."},
			"encoding":          map[string]any{"type": "string", "enum": []string{"utf-8"}, "description": "Explicit text encoding; only UTF-8 is supported."},
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
	if (arguments.Text == nil && arguments.Regex == nil) && (arguments.MaxMatchesPerFile != nil || arguments.MaxMatches != nil || arguments.ContextLines != nil || arguments.BinaryMode != nil || arguments.Encoding != nil) {
		return nil, invalidParamsError()
	}
	if arguments.BinaryMode != nil && *arguments.BinaryMode == "explicit" && arguments.ContextLines != nil {
		return nil, invalidParamsError()
	}
	limits, rpcErr := arguments.contentSearchLimits()
	if rpcErr != nil {
		return nil, rpcErr
	}
	if arguments.Text != nil {
		result, err := t.service.SearchLiteral(ctx, startPath, selector, kind, filter, *arguments.Text, limits)
		if err != nil {
			return nil, mapSearchError(err)
		}
		return result, nil
	}
	if arguments.Regex != nil {
		result, err := t.service.SearchRegex(ctx, startPath, selector, kind, filter, *arguments.Regex, limits)
		if err != nil {
			return nil, mapSearchError(err)
		}
		return result, nil
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
	MaxMatchesPerFile *int    `json:"maxMatchesPerFile,omitempty"`
	MaxMatches        *int    `json:"maxMatches,omitempty"`
	ContextLines      *int    `json:"contextLines,omitempty"`
	BinaryMode        *string `json:"binaryMode,omitempty"`
	Encoding          *string `json:"encoding,omitempty"`
}

func (a searchPathsArguments) contentSearchLimits() (search.LiteralLimits, *protocol.Error) {
	limits := search.LiteralLimits{
		MaxFiles: maxSearchContentFiles, MaxBytesPerFile: maxSearchBytesPerFile,
		MaxScannedBytes: maxSearchScannedBytes, MaxMatchesPerFile: maxSearchMatchesPerFile,
		MaxMatches: maxSearchContentMatches, MaxResponseBytes: maxSearchResponseBytes,
		MaxContextBytesPerMatch: maxSearchContextBytesPerMatch, MaxContextBytes: maxSearchContextBytes,
	}
	if a.MaxMatchesPerFile != nil {
		if *a.MaxMatchesPerFile <= 0 || *a.MaxMatchesPerFile > maxSearchMatchesPerFile {
			return search.LiteralLimits{}, invalidParamsError()
		}
		limits.MaxMatchesPerFile = *a.MaxMatchesPerFile
	}
	if a.MaxMatches != nil {
		if *a.MaxMatches <= 0 || *a.MaxMatches > maxSearchContentMatches {
			return search.LiteralLimits{}, invalidParamsError()
		}
		limits.MaxMatches = *a.MaxMatches
	}
	if a.ContextLines != nil {
		if *a.ContextLines <= 0 || *a.ContextLines > maxSearchContextLines {
			return search.LiteralLimits{}, invalidParamsError()
		}
		limits.ContextLines = *a.ContextLines
	}
	if a.Encoding != nil && *a.Encoding != "utf-8" {
		return search.LiteralLimits{}, invalidParamsError()
	}
	if a.BinaryMode != nil {
		switch *a.BinaryMode {
		case "skip":
			limits.BinaryMode = search.BinarySkip
		case "error":
			limits.BinaryMode = search.BinaryError
		case "explicit":
			limits.BinaryMode = search.BinaryExplicit
		default:
			return search.LiteralLimits{}, invalidParamsError()
		}
	}
	return limits, nil
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

func mapSearchError(err error) *protocol.Error {
	switch {
	case errors.Is(err, search.ErrInvalidNameSelector), errors.Is(err, search.ErrInvalidMetadataFilter), errors.Is(err, search.ErrInvalidLiteralSearch), errors.Is(err, search.ErrInvalidRegexSearch):
		return invalidParamsError()
	case errors.Is(err, search.ErrLimitExceeded), errors.Is(err, search.ErrTraversalLimitExceeded), errors.Is(err, search.ErrScanLimitExceeded), errors.Is(err, search.ErrResponseLimitExceeded), errors.Is(err, search.ErrContextLimitExceeded):
		return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: limit exceeded"}
	case errors.Is(err, fs.ErrFileTooLarge):
		return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: limit exceeded"}
	case errors.Is(err, search.ErrContentSearchUnavailable):
		return &protocol.Error{Code: protocol.ErrInternalError, Message: "search error: unavailable"}
	case errors.Is(err, search.ErrContextUnavailable):
		return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: context unavailable"}
	case errors.Is(err, search.ErrUnsupportedContent):
		return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "search error: unsupported content or encoding"}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return &protocol.Error{Code: protocol.ErrInternalError, Message: "search error: canceled"}
	default:
		return mapFilesystemError(err)
	}
}
