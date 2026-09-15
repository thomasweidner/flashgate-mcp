// Package search implements bounded, read-only search operations over the
// central filesystem abstraction.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

var (
	// ErrInvalidLimit is returned when a path search has a non-positive server cap.
	ErrInvalidLimit = errors.New("path search limits must be positive")
	// ErrLimitExceeded is returned before a result could exceed its server cap.
	ErrLimitExceeded = errors.New("path search limit exceeded")
	// ErrTraversalLimitExceeded is returned before traversal could exceed the
	// server-owned depth or visited-entry budget.
	ErrTraversalLimitExceeded = errors.New("search traversal limit exceeded")
	// ErrInvalidNameSelector is returned before traversal when a filename
	// selector is empty, contains a path separator, or has invalid pattern syntax.
	ErrInvalidNameSelector = errors.New("invalid filename selector")
	// ErrInvalidMetadataFilter is returned before traversal when metadata filter
	// bounds or the requested portable entry type are invalid.
	ErrInvalidMetadataFilter = errors.New("invalid metadata filter")
	// ErrInvalidLiteralSearch is returned before traversal for an empty or
	// invalid UTF-8 term, incompatible filters, or non-positive server budgets.
	ErrInvalidLiteralSearch = errors.New("invalid literal text search")
	// ErrInvalidRegexSearch is returned before traversal for an empty, invalid,
	// or non-UTF-8 regular expression or incompatible search settings.
	ErrInvalidRegexSearch = errors.New("invalid regular-expression search")
	// ErrContentSearchUnavailable is returned when the filesystem boundary does
	// not provide root-confined reads.
	ErrContentSearchUnavailable = errors.New("content search unavailable")
	// ErrScanLimitExceeded is returned before content scanning could exceed a
	// server-owned file or byte budget.
	ErrScanLimitExceeded = errors.New("content search scan limit exceeded")
	// ErrResponseLimitExceeded is returned before an oversized match response is
	// returned to the adapter.
	ErrResponseLimitExceeded = errors.New("content search response limit exceeded")
)

// NameMatchKind identifies how a filename selector is interpreted.
type NameMatchKind uint8

const (
	NameMatchLiteral NameMatchKind = iota
	NameMatchPattern
)

// Path is one policy-visible entry found below the selected start path.
type Path struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// LiteralMatch identifies a non-overlapping literal occurrence by its
// root-relative file path and zero-based UTF-8 byte offset.
type LiteralMatch struct {
	Path       string `json:"path"`
	ByteOffset int64  `json:"byteOffset"`
}

// ContentSearchResult contains the bounded matches and reports when matching
// stopped at a client- or server-owned match budget.
type ContentSearchResult struct {
	Matches   []LiteralMatch        `json:"matches"`
	Truncated bool                  `json:"truncated"`
	Limit     *MatchLimitDiagnostic `json:"limit,omitempty"`
}

// MatchLimitDiagnostic identifies the match budget that stopped the search.
// Path is present only for a per-file limit and is always root-relative.
type MatchLimitDiagnostic struct {
	Kind            string `json:"kind"`
	MaxMatches      int    `json:"maxMatches"`
	ReturnedMatches int    `json:"returnedMatches"`
	Path            string `json:"path,omitempty"`
}

// LiteralLimits are mandatory server-owned budgets for one content search.
type LiteralLimits struct {
	MaxFiles          int
	MaxBytesPerFile   int64
	MaxScannedBytes   int64
	MaxMatchesPerFile int
	MaxMatches        int
	MaxResponseBytes  int
}

// EntryType is a portable classification of policy-visible search entries.
type EntryType uint8

const (
	EntryTypeAny EntryType = iota
	EntryTypeFile
	EntryTypeDirectory
)

// MetadataFilter narrows results using portable directory-entry metadata.
// Size bounds apply only to files. Time and size bounds are inclusive.
type MetadataFilter struct {
	Type              EntryType
	MinSizeBytes      *int64
	MaxSizeBytes      *int64
	ModifiedNotBefore *time.Time
	ModifiedNotAfter  *time.Time
}

// PathService recursively enumerates root-relative paths through the central
// filesystem boundary. It never receives or exposes an absolute host path.
type PathService struct {
	filesystem fs.DirectoryLister
	maxResults int
	maxDepth   int
	maxEntries int
}

// NewPathService creates a path search with mandatory server-owned result,
// recursion-depth, and visited-entry caps. Depth counts descendants below the
// selected start path, whose depth is zero.
func NewPathService(filesystem fs.DirectoryLister, maxResults, maxDepth, maxEntries int) (*PathService, error) {
	if maxResults <= 0 || maxDepth <= 0 || maxEntries <= 0 {
		return nil, ErrInvalidLimit
	}
	return &PathService{filesystem: filesystem, maxResults: maxResults, maxDepth: maxDepth, maxEntries: maxEntries}, nil
}

// Search returns every policy-visible descendant in deterministic UTF-8 byte
// order. The start path itself is not included.
func (s *PathService) Search(ctx context.Context, startPath string) ([]Path, error) {
	return s.SearchFiltered(ctx, startPath, "", NameMatchLiteral, MetadataFilter{})
}

// SearchNames returns descendants whose base filename matches selector. Literal
// matching is exact and case-sensitive. Pattern matching uses path.Match syntax,
// also case-sensitively, and matches the complete base filename. Directories that
// do not match are still traversed so matching descendants remain discoverable.
func (s *PathService) SearchNames(ctx context.Context, startPath, selector string, kind NameMatchKind) ([]Path, error) {
	if selector == "" {
		return nil, ErrInvalidNameSelector
	}
	return s.SearchFiltered(ctx, startPath, selector, kind, MetadataFilter{})
}

// SearchFiltered returns descendants matching optional filename and portable
// metadata filters. All filters are validated before filesystem traversal.
func (s *PathService) SearchFiltered(ctx context.Context, startPath, selector string, kind NameMatchKind, filter MetadataFilter) ([]Path, error) {
	if err := validateMetadataFilter(filter); err != nil {
		return nil, err
	}

	var nameMatches func(string) bool
	if selector != "" {
		if strings.Contains(selector, "/") {
			return nil, ErrInvalidNameSelector
		}

		nameMatches = func(name string) bool { return name == selector }
		if kind == NameMatchPattern {
			if _, err := path.Match(selector, ""); err != nil {
				return nil, ErrInvalidNameSelector
			}
			nameMatches = func(name string) bool {
				matched, _ := path.Match(selector, name)
				return matched
			}
		} else if kind != NameMatchLiteral {
			return nil, ErrInvalidNameSelector
		}
	} else if kind != NameMatchLiteral {
		return nil, ErrInvalidNameSelector
	}

	return s.search(ctx, startPath, func(entry fs.Entry) bool {
		return (nameMatches == nil || nameMatches(entry.Name)) && matchesMetadata(entry, filter)
	})
}

// SearchLiteral returns non-overlapping, case-sensitive literal matches in
// deterministic path/byte-offset order. Content is matched as bytes after the
// selector is validated as UTF-8; binary classification and alternate
// encodings remain outside this baseline operation.
func (s *PathService) SearchLiteral(ctx context.Context, startPath, selector string, kind NameMatchKind, filter MetadataFilter, term string, limits LiteralLimits) (ContentSearchResult, error) {
	if term == "" || !utf8.ValidString(term) || filter.Type == EntryTypeDirectory ||
		limits.MaxFiles <= 0 || limits.MaxBytesPerFile <= 0 || limits.MaxScannedBytes <= 0 ||
		limits.MaxMatchesPerFile <= 0 || limits.MaxMatches <= 0 || limits.MaxResponseBytes <= 0 {
		return ContentSearchResult{}, ErrInvalidLiteralSearch
	}
	needle := []byte(term)
	return s.searchContent(ctx, startPath, selector, kind, filter, limits, func(content []byte, maxMatches int) [][2]int {
		var offsets [][2]int
		for base := 0; base <= len(content)-len(needle); {
			offset := bytes.Index(content[base:], needle)
			if offset < 0 {
				break
			}
			absolute := base + offset
			offsets = append(offsets, [2]int{absolute, absolute + len(needle)})
			if len(offsets) == maxMatches {
				break
			}
			base = absolute + len(needle)
		}
		return offsets
	})
}

// SearchRegex returns non-overlapping matches produced by Go's RE2-style,
// linear-time regular-expression engine. The same server-owned scan, match,
// response, and cancellation budgets as literal content search are enforced.
func (s *PathService) SearchRegex(ctx context.Context, startPath, selector string, kind NameMatchKind, filter MetadataFilter, expression string, limits LiteralLimits) (ContentSearchResult, error) {
	if expression == "" || !utf8.ValidString(expression) || filter.Type == EntryTypeDirectory || !validLiteralLimits(limits) {
		return ContentSearchResult{}, ErrInvalidRegexSearch
	}
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return ContentSearchResult{}, ErrInvalidRegexSearch
	}
	return s.searchContent(ctx, startPath, selector, kind, filter, limits, func(content []byte, maxMatches int) [][2]int {
		indexes := compiled.FindAllIndex(content, maxMatches)
		offsets := make([][2]int, len(indexes))
		for i, index := range indexes {
			offsets[i] = [2]int{index[0], index[1]}
		}
		return offsets
	})
}

func validLiteralLimits(limits LiteralLimits) bool {
	return limits.MaxFiles > 0 && limits.MaxBytesPerFile > 0 && limits.MaxScannedBytes > 0 &&
		limits.MaxMatchesPerFile > 0 && limits.MaxMatches > 0 && limits.MaxResponseBytes > 0
}

func (s *PathService) searchContent(ctx context.Context, startPath, selector string, kind NameMatchKind, filter MetadataFilter, limits LiteralLimits, find func([]byte, int) [][2]int) (ContentSearchResult, error) {
	if err := validateMetadataFilter(filter); err != nil {
		return ContentSearchResult{}, err
	}
	nameMatches, err := compileNameMatcher(selector, kind)
	if err != nil {
		return ContentSearchResult{}, err
	}
	reader, ok := s.filesystem.(fs.ContentReader)
	if !ok {
		return ContentSearchResult{}, ErrContentSearchUnavailable
	}

	startPath = normalizeRelativePath(startPath)
	type pendingDirectory struct {
		path  string
		depth int
	}
	pending := []pendingDirectory{{path: startPath}}
	matches := make([]LiteralMatch, 0)
	files, entriesVisited, scanned := 0, 0, int64(0)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return ContentSearchResult{}, err
		}
		directory := pending[0]
		pending = pending[1:]
		entries, err := s.filesystem.List(directory.path)
		if err != nil {
			return ContentSearchResult{}, err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return ContentSearchResult{}, err
			}
			if entriesVisited == s.maxEntries {
				return ContentSearchResult{}, ErrTraversalLimitExceeded
			}
			entriesVisited++
			relativePath := joinRelativePath(directory.path, entry.Name)
			if entry.IsDir {
				if directory.depth == s.maxDepth-1 {
					return ContentSearchResult{}, ErrTraversalLimitExceeded
				}
				pending = append(pending, pendingDirectory{path: relativePath, depth: directory.depth + 1})
				continue
			}
			if (nameMatches != nil && !nameMatches(entry.Name)) || !matchesMetadata(entry, filter) {
				continue
			}
			if files == limits.MaxFiles || entry.Size > limits.MaxBytesPerFile || entry.Size > limits.MaxScannedBytes-scanned {
				return ContentSearchResult{}, ErrScanLimitExceeded
			}
			content, err := reader.Read(relativePath, limits.MaxBytesPerFile)
			if err != nil {
				return ContentSearchResult{}, err
			}
			if int64(len(content)) > limits.MaxBytesPerFile || int64(len(content)) > limits.MaxScannedBytes-scanned {
				return ContentSearchResult{}, ErrScanLimitExceeded
			}
			files++
			scanned += int64(len(content))
			fileMatches := 0
			maxFound := limits.MaxMatchesPerFile + 1
			if remaining := limits.MaxMatches - len(matches) + 1; remaining < maxFound {
				maxFound = remaining
			}
			for _, offset := range find(content, maxFound) {
				if err := ctx.Err(); err != nil {
					return ContentSearchResult{}, err
				}
				if fileMatches == limits.MaxMatchesPerFile {
					return boundedContentResult(matches, limits.MaxResponseBytes, &MatchLimitDiagnostic{
						Kind: "perFileMatches", MaxMatches: limits.MaxMatchesPerFile, ReturnedMatches: fileMatches, Path: relativePath,
					})
				}
				if len(matches) == limits.MaxMatches {
					return boundedContentResult(matches, limits.MaxResponseBytes, &MatchLimitDiagnostic{
						Kind: "totalMatches", MaxMatches: limits.MaxMatches, ReturnedMatches: len(matches),
					})
				}
				matches = append(matches, LiteralMatch{Path: relativePath, ByteOffset: int64(offset[0])})
				fileMatches++
				encoded, _ := json.Marshal(ContentSearchResult{Matches: matches})
				if len(encoded) > limits.MaxResponseBytes {
					return ContentSearchResult{}, ErrResponseLimitExceeded
				}
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path == matches[j].Path {
			return matches[i].ByteOffset < matches[j].ByteOffset
		}
		return matches[i].Path < matches[j].Path
	})
	return ContentSearchResult{Matches: matches}, nil
}

func boundedContentResult(matches []LiteralMatch, maxResponseBytes int, limit *MatchLimitDiagnostic) (ContentSearchResult, error) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path == matches[j].Path {
			return matches[i].ByteOffset < matches[j].ByteOffset
		}
		return matches[i].Path < matches[j].Path
	})
	result := ContentSearchResult{Matches: matches, Truncated: true, Limit: limit}
	encoded, _ := json.Marshal(result)
	if len(encoded) > maxResponseBytes {
		return ContentSearchResult{}, ErrResponseLimitExceeded
	}
	return result, nil
}

func compileNameMatcher(selector string, kind NameMatchKind) (func(string) bool, error) {
	if selector == "" {
		if kind != NameMatchLiteral {
			return nil, ErrInvalidNameSelector
		}
		return nil, nil
	}
	if strings.Contains(selector, "/") {
		return nil, ErrInvalidNameSelector
	}
	if kind == NameMatchLiteral {
		return func(name string) bool { return name == selector }, nil
	}
	if kind != NameMatchPattern {
		return nil, ErrInvalidNameSelector
	}
	if _, err := path.Match(selector, ""); err != nil {
		return nil, ErrInvalidNameSelector
	}
	return func(name string) bool { matched, _ := path.Match(selector, name); return matched }, nil
}

func validateMetadataFilter(filter MetadataFilter) error {
	if filter.Type > EntryTypeDirectory ||
		(filter.MinSizeBytes != nil && *filter.MinSizeBytes < 0) ||
		(filter.MaxSizeBytes != nil && *filter.MaxSizeBytes < 0) ||
		(filter.MinSizeBytes != nil && filter.MaxSizeBytes != nil && *filter.MinSizeBytes > *filter.MaxSizeBytes) ||
		(filter.ModifiedNotBefore != nil && filter.ModifiedNotAfter != nil && filter.ModifiedNotBefore.After(*filter.ModifiedNotAfter)) {
		return ErrInvalidMetadataFilter
	}
	return nil
}

func matchesMetadata(entry fs.Entry, filter MetadataFilter) bool {
	if (filter.Type == EntryTypeFile && entry.IsDir) || (filter.Type == EntryTypeDirectory && !entry.IsDir) {
		return false
	}
	if filter.MinSizeBytes != nil || filter.MaxSizeBytes != nil {
		if entry.IsDir || (filter.MinSizeBytes != nil && entry.Size < *filter.MinSizeBytes) || (filter.MaxSizeBytes != nil && entry.Size > *filter.MaxSizeBytes) {
			return false
		}
	}
	if (filter.ModifiedNotBefore != nil && entry.ModifiedTime.Before(*filter.ModifiedNotBefore)) ||
		(filter.ModifiedNotAfter != nil && entry.ModifiedTime.After(*filter.ModifiedNotAfter)) {
		return false
	}
	return true
}

func (s *PathService) search(ctx context.Context, startPath string, matches func(fs.Entry) bool) ([]Path, error) {
	startPath = normalizeRelativePath(startPath)
	results := make([]Path, 0)
	type pendingDirectory struct {
		path  string
		depth int
	}
	pending := []pendingDirectory{{path: startPath}}
	entriesVisited := 0

	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		directory := pending[0]
		pending = pending[1:]
		entries, err := s.filesystem.List(directory.path)
		if err != nil {
			return nil, err
		}

		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if entriesVisited == s.maxEntries {
				return nil, ErrTraversalLimitExceeded
			}
			entriesVisited++
			relativePath := joinRelativePath(directory.path, entry.Name)
			if matches(entry) {
				if len(results) == s.maxResults {
					return nil, ErrLimitExceeded
				}
				results = append(results, Path{Path: relativePath, IsDir: entry.IsDir})
			}
			if entry.IsDir {
				if directory.depth == s.maxDepth-1 {
					return nil, ErrTraversalLimitExceeded
				}
				pending = append(pending, pendingDirectory{path: relativePath, depth: directory.depth + 1})
			}
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, nil
}

func normalizeRelativePath(value string) string {
	value = strings.ReplaceAll(value, `\`, "/")
	if value == "" {
		return "."
	}
	return value
}

func joinRelativePath(directory, name string) string {
	if directory == "." {
		return path.Clean(name)
	}
	return path.Join(directory, name)
}
