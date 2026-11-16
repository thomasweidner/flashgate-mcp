// Package roots binds configured root identities to confined filesystems.
package roots

import (
	"errors"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// ErrUnknownRoot denotes an ID that is not registered exactly as supplied.
var ErrUnknownRoot = errors.New("unknown root id")

// Registry is an immutable map of configured root IDs to backends. A lookup
// selects confinement. The caller remains responsible for capability checks.
type Registry struct {
	defaultID config.RootID
	ids       []config.RootID
	entries   map[config.RootID]fs.FileSystem
}

// New validates and binds every root before returning the registry.
func New(cfg config.Config) (*Registry, error) {
	files := cfg.Files()
	definitions := files.Roots()
	policy := security.Policy{
		AllowHiddenFiles: cfg.Security().AllowHiddenFiles(),
		AllowUNCPaths:    cfg.Security().AllowUNCPaths(),
		FollowSymlinks:   cfg.Security().FollowSymlinks(),
	}
	guards := make([]*security.PathGuard, 0, len(definitions))
	for _, def := range definitions {
		guard, err := security.NewPathGuardWithPolicy(def.Path, policy)
		if err != nil {
			return nil, err
		}
		guards = append(guards, guard)
	}
	bindings, err := cfg.ProtectDataRoots(guards)
	if err != nil {
		return nil, err
	}
	backends, err := fs.NewProtectedFileSystems(bindings, fs.Limits{
		MaxWriteBytes:    files.MaxWriteBytes(),
		MaxListEntries:   files.MaxListEntries(),
		MaxCopyBytes:     files.MaxCopyBytes(),
		MaxDeleteEntries: files.MaxDeleteEntries(),
	})
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		defaultID: files.DefaultID(),
		ids:       make([]config.RootID, 0, len(definitions)),
		entries:   make(map[config.RootID]fs.FileSystem, len(definitions)),
	}
	for i, def := range definitions {
		registry.ids = append(registry.ids, def.ID)
		registry.entries[def.ID] = backends[i]
	}
	return registry, nil
}

// DefaultID returns the explicitly configured default root.
func (r *Registry) DefaultID() config.RootID { return r.defaultID }

// IDs returns an independent snapshot in canonical ID order.
func (r *Registry) IDs() []config.RootID { return slices.Clone(r.ids) }

// Lookup returns the backend bound to an exact configured ID.
func (r *Registry) Lookup(id config.RootID) (fs.FileSystem, error) {
	backend, ok := r.entries[id]
	if !ok {
		return nil, ErrUnknownRoot
	}
	return backend, nil
}
