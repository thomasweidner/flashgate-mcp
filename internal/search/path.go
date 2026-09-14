// Package search implements bounded, read-only search operations over the
// central filesystem abstraction.
package search

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

var (
	// ErrInvalidLimit is returned when a path search has no positive server cap.
	ErrInvalidLimit = errors.New("path search limit must be positive")
	// ErrLimitExceeded is returned before a result could exceed its server cap.
	ErrLimitExceeded = errors.New("path search limit exceeded")
	// ErrInvalidNameSelector is returned before traversal when a filename
	// selector is empty, contains a path separator, or has invalid pattern syntax.
	ErrInvalidNameSelector = errors.New("invalid filename selector")
	// ErrInvalidMetadataFilter is returned before traversal when metadata filter
	// bounds or the requested portable entry type are invalid.
	ErrInvalidMetadataFilter = errors.New("invalid metadata filter")
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
}

// NewPathService creates a path search with a mandatory server-owned cap.
func NewPathService(filesystem fs.DirectoryLister, maxResults int) (*PathService, error) {
	if maxResults <= 0 {
		return nil, ErrInvalidLimit
	}
	return &PathService{filesystem: filesystem, maxResults: maxResults}, nil
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
	pending := []string{startPath}

	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		directory := pending[0]
		pending = pending[1:]
		entries, err := s.filesystem.List(directory)
		if err != nil {
			return nil, err
		}

		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			relativePath := joinRelativePath(directory, entry.Name)
			if matches(entry) {
				if len(results) == s.maxResults {
					return nil, ErrLimitExceeded
				}
				results = append(results, Path{Path: relativePath, IsDir: entry.IsDir})
			}
			if entry.IsDir {
				pending = append(pending, relativePath)
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
