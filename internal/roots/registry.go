// Package roots owns the application-level mapping from opaque root IDs to
// root-confined filesystem implementations.
package roots

import (
	"errors"
	"sort"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

const DefaultID = "default"

var (
	ErrInvalidEntry = errors.New("invalid root entry")
	ErrDuplicateID  = errors.New("duplicate root id")
	ErrNoRoots      = errors.New("no roots configured")
	ErrUnknownRoot  = errors.New("unknown root id")
)

type Entry struct {
	ID         string
	FileSystem fs.FileSystem
}

// Registry is an immutable collection of named, independently confined roots.
type Registry struct {
	entries map[string]fs.FileSystem
}

func New(entries []Entry) (*Registry, error) {
	if len(entries) == 0 {
		return nil, ErrNoRoots
	}
	registry := &Registry{entries: make(map[string]fs.FileSystem, len(entries))}
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" || entry.FileSystem == nil {
			return nil, ErrInvalidEntry
		}
		if _, exists := registry.entries[entry.ID]; exists {
			return nil, ErrDuplicateID
		}
		registry.entries[entry.ID] = entry.FileSystem
	}
	return registry, nil
}

// Single preserves the current single-root deployment as the default named root.
func Single(filesystem fs.FileSystem) (*Registry, error) {
	return New([]Entry{{ID: DefaultID, FileSystem: filesystem}})
}

func (r *Registry) FileSystem(id string) (fs.FileSystem, error) {
	filesystem, ok := r.entries[id]
	if !ok {
		return nil, ErrUnknownRoot
	}
	return filesystem, nil
}

func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.entries))
	for id := range r.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
