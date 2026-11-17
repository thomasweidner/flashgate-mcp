// Package roots binds configured root identities to confined filesystems.
package roots

import (
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Registry binds each configured root ID to its access policy and confined
// backend. The caller remains responsible for capability checks.
type Registry struct {
	ids     []config.RootID
	entries map[config.RootID]entry
}

type entry struct {
	access  config.RootAccess
	backend *fs.LocalFileSystem
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
		ids:     make([]config.RootID, 0, len(definitions)),
		entries: make(map[config.RootID]entry, len(definitions)),
	}
	for i, def := range definitions {
		registry.ids = append(registry.ids, def.ID)
		registry.entries[def.ID] = entry{access: def.Access, backend: backends[i]}
	}
	return registry, nil
}

// IDs returns an independent snapshot in canonical ID order.
func (r *Registry) IDs() []config.RootID { return slices.Clone(r.ids) }

func (r *Registry) backendFor(id config.RootID, required config.RootAccess) (*fs.LocalFileSystem, error) {
	root, ok := r.entries[id]
	if !ok {
		return nil, fs.ErrUnknownRoot
	}
	if !root.access.Allows(required) {
		return nil, fs.ErrRootAccessDenied
	}
	return root.backend, nil
}
