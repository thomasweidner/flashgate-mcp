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
	ErrAccessDenied = errors.New("root access denied")
)

// Access is a set of operations permitted for a named root.
type Access uint8

const (
	Read Access = 1 << iota
	Write
	ReadWrite = Read | Write
)

type Entry struct {
	ID         string
	FileSystem fs.FileSystem
	Access     Access
}

// Registry is an immutable collection of named, independently confined roots.
type Registry struct {
	entries map[string]Entry
}

func New(entries []Entry) (*Registry, error) {
	if len(entries) == 0 {
		return nil, ErrNoRoots
	}
	registry := &Registry{entries: make(map[string]Entry, len(entries))}
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" || entry.FileSystem == nil ||
			entry.Access == 0 || entry.Access&^ReadWrite != 0 {
			return nil, ErrInvalidEntry
		}
		if _, exists := registry.entries[entry.ID]; exists {
			return nil, ErrDuplicateID
		}
		registry.entries[entry.ID] = entry
	}
	return registry, nil
}

// Single preserves the current single-root deployment as the default named root.
func Single(filesystem fs.FileSystem) (*Registry, error) {
	return SingleWithAccess(filesystem, ReadWrite)
}

// SingleWithAccess preserves the current single-root deployment with an
// explicit policy derived from its effective profile.
func SingleWithAccess(filesystem fs.FileSystem, access Access) (*Registry, error) {
	return New([]Entry{{ID: DefaultID, FileSystem: filesystem, Access: access}})
}

// FileSystem returns a root only when all requested access is permitted.
func (r *Registry) FileSystem(id string, required Access) (fs.FileSystem, error) {
	entry, ok := r.entries[id]
	if !ok {
		return nil, ErrUnknownRoot
	}
	if required == 0 || required&^ReadWrite != 0 || entry.Access&required != required {
		return nil, ErrAccessDenied
	}
	return entry.FileSystem, nil
}

func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.entries))
	for id := range r.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
