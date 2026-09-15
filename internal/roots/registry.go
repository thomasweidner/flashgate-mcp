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
	ErrInvalidEntry  = errors.New("invalid root entry")
	ErrDuplicateID   = errors.New("duplicate root id")
	ErrNoRoots       = errors.New("no roots configured")
	ErrUnknownRoot   = errors.New("unknown root id")
	ErrAccessDenied  = errors.New("root access denied")
	ErrInvalidLimits = errors.New("invalid root limits")
)

// Access is a set of operations permitted for a named root.
type Access uint8

const (
	Read Access = 1 << iota
	Write
	ReadWrite = Read | Write
)

// Limits bounds data handled on behalf of one named root. Limits for features
// that are not implemented yet are retained here so future scan and temporary
// data paths cannot accidentally fall back to an unbounded policy.
type Limits struct {
	MaxFileBytes      int64
	MaxResultBytes    int64
	MaxScanBytes      int64
	MaxTemporaryBytes int64
}

// DefaultLimits preserves the limits used by the compatible single-root path.
func DefaultLimits(maxFileBytes, maxResultBytes int64) Limits {
	return Limits{
		MaxFileBytes:      maxFileBytes,
		MaxResultBytes:    maxResultBytes,
		MaxScanBytes:      maxFileBytes,
		MaxTemporaryBytes: maxFileBytes,
	}
}

func (l Limits) valid() bool {
	return l.MaxFileBytes > 0 && l.MaxResultBytes > 0 &&
		l.MaxScanBytes > 0 && l.MaxTemporaryBytes > 0
}

type Entry struct {
	ID         string
	FileSystem fs.FileSystem
	Access     Access
	Limits     Limits
}

// Root is the resolved, authorized policy and filesystem for one root.
type Root struct {
	FileSystem fs.FileSystem
	Limits     Limits
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
		if !entry.Limits.valid() {
			return nil, ErrInvalidLimits
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
	return SingleWithPolicy(filesystem, ReadWrite, DefaultLimits(10*1024*1024, 16*1024*1024))
}

// SingleWithAccess preserves the current single-root deployment with an
// explicit policy derived from its effective profile.
func SingleWithAccess(filesystem fs.FileSystem, access Access) (*Registry, error) {
	return SingleWithPolicy(filesystem, access, DefaultLimits(10*1024*1024, 16*1024*1024))
}

// SingleWithPolicy preserves the current single-root deployment with explicit
// access and resource policies derived from configuration.
func SingleWithPolicy(filesystem fs.FileSystem, access Access, limits Limits) (*Registry, error) {
	return New([]Entry{{ID: DefaultID, FileSystem: filesystem, Access: access, Limits: limits}})
}

// FileSystem returns a root only when all requested access is permitted.
func (r *Registry) FileSystem(id string, required Access) (fs.FileSystem, error) {
	root, err := r.Root(id, required)
	return root.FileSystem, err
}

// Root returns the complete root policy only when access is permitted.
func (r *Registry) Root(id string, required Access) (Root, error) {
	entry, ok := r.entries[id]
	if !ok {
		return Root{}, ErrUnknownRoot
	}
	if required == 0 || required&^ReadWrite != 0 || entry.Access&required != required {
		return Root{}, ErrAccessDenied
	}
	return Root{FileSystem: entry.FileSystem, Limits: entry.Limits}, nil
}

func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.entries))
	for id := range r.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
