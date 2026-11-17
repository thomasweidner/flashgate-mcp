// Package roots binds configured root identities to confined filesystems.
package roots

import (
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Registry binds each configured root ID to its access policy and confined
// backend. The caller remains responsible for capability checks.
type Registry struct {
	ids     []config.RootID
	entries map[config.RootID]entry
}

type entry struct {
	budget  *resource.Meter
	limits  resource.Limits
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
	limits := make([]fs.Limits, len(definitions))
	for i, def := range definitions {
		l := def.Limits
		limits[i] = fs.Limits{MaxFileSize: l.File, MaxWriteSize: l.Write, MaxScanSize: l.Scan,
			MaxResultSize: l.Result, MaxTempSize: l.Temp, MaxListEntries: files.MaxListEntries(), MaxDeleteEntries: files.MaxDeleteEntries()}
	}
	backends, err := fs.NewProtectedRootFileSystems(bindings, limits)
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		ids:     make([]config.RootID, 0, len(definitions)),
		entries: make(map[config.RootID]entry, len(definitions)),
	}
	for i, def := range definitions {
		budget, err := resource.NewMeter(def.Limits)
		if err != nil {
			return nil, err
		}
		registry.ids = append(registry.ids, def.ID)
		registry.entries[def.ID] = entry{access: def.Access, limits: def.Limits, budget: budget, backend: backends[i]}
	}
	return registry, nil
}

// LimitsForRoot returns the immutable effective value for an exact root ID.
// This grants no access and never exposes a backend or path.
func (r *Registry) LimitsForRoot(id config.RootID) (resource.Limits, error) {
	root, ok := r.entries[id]
	if !ok {
		return resource.Limits{}, fs.ErrUnknownRoot
	}
	return root.limits, nil
}

// ReserveTemp enforces the aggregate live scratch ceiling for this exact root
// across concurrent operations. It grants no filesystem or backend access.
// Consumers also enforce any tighter operation/request allowance and defer
// release before allocating scratch. Aliased IDs retain separate budgets.
func (r *Registry) ReserveTemp(id config.RootID, n int64) (func(), error) {
	root, ok := r.entries[id]
	if !ok {
		return nil, fs.ErrUnknownRoot
	}
	return root.budget.ReserveTemp(n)
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
