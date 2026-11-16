package security

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// ErrConfigurationAccess denies an ordinary effect on the protected
// configuration boundary or on an operand whose protection is unprovable.
var ErrConfigurationAccess = errors.New("protected configuration access denied")

// ConfigProtection binds one running snapshot to its loaded file, selected
// pathname and original ancestors. The selected name can be rebound by the OS.
type ConfigProtection struct {
	source  string
	loaded  os.FileInfo
	parents []boundDirectory
}

// RootProtection binds a guarded root to a configuration protection snapshot.
type RootProtection struct {
	config       *ConfigProtection
	guard        *PathGuard
	checkAliases func() error
}

type boundDirectory struct {
	path string
	info os.FileInfo
}

// BindConfigProtection records the configuration identity and its ancestors.
func BindConfigProtection(source string, loaded os.FileInfo) (*ConfigProtection, error) {
	if !filepath.IsAbs(source) || loaded == nil {
		return nil, ErrProtectedPath
	}
	parents, err := configParents(source)
	if err != nil || parents[0].info == nil {
		return nil, ErrProtectedPath
	}
	current, err := PathIdentity(source)
	if err != nil || !os.SameFile(loaded, current) {
		return nil, ErrProtectedPath
	}
	return &ConfigProtection{source: source, loaded: loaded, parents: parents}, nil
}

// BindRoot verifies the root identity and the native alias checks at startup.
func (p *ConfigProtection) BindRoot(guard *PathGuard, checkAliases func() error) (*RootProtection, error) {
	if p == nil || p.loaded == nil || guard == nil || checkAliases == nil {
		return nil, ErrProtectedPath
	}
	binding := &RootProtection{config: p, guard: guard, checkAliases: checkAliases}
	if err := binding.ValidateRoot(); err != nil {
		return nil, ErrProtectedPath
	}
	return binding, nil
}

// Guard returns the immutable PathGuard used with this binding.
func (r *RootProtection) Guard() *PathGuard { return r.guard }

func configParents(source string) ([]boundDirectory, error) {
	var parents []boundDirectory
	for dir := filepath.Dir(source); ; dir = filepath.Dir(dir) {
		info, err := PathIdentity(dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, ErrConfigurationAccess
		}
		parents = append(parents, boundDirectory{path: dir, info: info})
		if filepath.Dir(dir) == dir {
			return parents, nil
		}
	}
}

// ValidateRoot rechecks the authorized effective data root and the native
// alias assumptions for this operation.
func (r *RootProtection) ValidateRoot() error {
	g := r.guard
	resolved, err := evalEffectivePath(g.root)
	if err != nil || !configPathEqual(resolved, g.effectiveRoot) {
		return ErrConfigurationAccess
	}
	info, err := PathIdentity(resolved)
	if err != nil || !sameIdentity(info, g.rootInfo) || r.checkAliases() != nil {
		return ErrConfigurationAccess
	}
	return nil
}

// Check protects the selected name and both the loaded and current file
// objects. A mutation additionally protects ancestors of the selected name.
// info is nil for an absent operand.
func (r *RootProtection) Check(path string, info, parent os.FileInfo, mutation bool) error {
	p := r.config
	if isInsideRoot(p.source, path) || sameIdentity(info, p.loaded) {
		return ErrConfigurationAccess
	}
	current, err := PathIdentity(p.source)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrConfigurationAccess
	}
	if sameIdentity(info, current) {
		return ErrConfigurationAccess
	}
	if configNameEqual(filepath.Base(path), filepath.Base(p.source)) &&
		(configPathEqual(filepath.Dir(path), p.parents[0].path) || sameIdentity(parent, p.parents[0].info)) {
		return ErrConfigurationAccess
	}
	if !mutation {
		return nil
	}
	for _, dir := range p.parents {
		if configPathEqual(path, dir.path) {
			return ErrConfigurationAccess
		}
	}
	if info != nil && !info.IsDir() {
		return nil
	}
	parents, err := configParents(p.source)
	if err != nil {
		return err
	}
	// Linux filesystems can fold and normalize names during lookup. When a
	// protected name is absent, pathname spelling and file identity cannot
	// establish that a new destination in the same directory is distinct.
	unprovedLinuxName := runtime.GOOS == "linux" && info == nil
	if unprovedLinuxName && current == nil &&
		(sameIdentity(parent, parents[0].info) || sameIdentity(parent, p.parents[0].info)) {
		return ErrConfigurationAccess
	}
	if sameIdentity(parent, parents[0].info) && configNameEqual(filepath.Base(path), filepath.Base(p.source)) {
		return ErrConfigurationAccess
	}
	for i, dir := range parents {
		if sameIdentity(info, dir.info) || sameIdentity(info, p.parents[i].info) {
			return ErrConfigurationAccess
		}
		if unprovedLinuxName && i > 0 && parents[i-1].info == nil && sameIdentity(parent, dir.info) {
			return ErrConfigurationAccess
		}
	}
	return nil
}

func sameIdentity(left, right os.FileInfo) bool {
	return left != nil && right != nil && os.SameFile(left, right)
}
