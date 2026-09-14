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
			if len(results) == s.maxResults {
				return nil, ErrLimitExceeded
			}

			relativePath := joinRelativePath(directory, entry.Name)
			results = append(results, Path{Path: relativePath, IsDir: entry.IsDir})
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
