// Package search implements bounded, read-only search operations over the
// central filesystem abstraction.
package search

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"

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
	return s.search(ctx, startPath, nil)
}

// SearchNames returns descendants whose base filename matches selector. Literal
// matching is exact and case-sensitive. Pattern matching uses path.Match syntax,
// also case-sensitively, and matches the complete base filename. Directories that
// do not match are still traversed so matching descendants remain discoverable.
func (s *PathService) SearchNames(ctx context.Context, startPath, selector string, kind NameMatchKind) ([]Path, error) {
	if selector == "" || strings.Contains(selector, "/") {
		return nil, ErrInvalidNameSelector
	}

	matcher := func(name string) bool { return name == selector }
	if kind == NameMatchPattern {
		if _, err := path.Match(selector, ""); err != nil {
			return nil, ErrInvalidNameSelector
		}
		matcher = func(name string) bool {
			matched, _ := path.Match(selector, name)
			return matched
		}
	} else if kind != NameMatchLiteral {
		return nil, ErrInvalidNameSelector
	}

	return s.search(ctx, startPath, matcher)
}

func (s *PathService) search(ctx context.Context, startPath string, matches func(string) bool) ([]Path, error) {
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
			if matches == nil || matches(entry.Name) {
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
