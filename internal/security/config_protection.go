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
	source       string
	loaded       os.FileInfo
	parents      []boundDirectory
	root         os.FileInfo
	validateRoot func() error
}

type boundDirectory struct {
	path string
	info os.FileInfo
}

// BindConfigProtection records the configuration and root identities before
// the filesystem can be exposed to callers.
func (g *PathGuard) BindConfigProtection(source string, loaded os.FileInfo, validateRoot func() error) (*ConfigProtection, error) {
	if g == nil || !filepath.IsAbs(source) || loaded == nil || validateRoot == nil {
		return nil, ErrProtectedPath
	}
	root, err := PathIdentity(g.effectiveRoot)
	if err != nil || !root.IsDir() {
		return nil, ErrProtectedPath
	}
	p := &ConfigProtection{source: source, loaded: loaded, root: root, validateRoot: validateRoot}
	p.parents, err = configParents(source)
	if err != nil || len(p.parents) == 0 || p.parents[0].info == nil {
		return nil, ErrProtectedPath
	}
	current, err := PathIdentity(source)
	if err != nil || !os.SameFile(loaded, current) {
		return nil, ErrProtectedPath
	}
	return p, nil
}

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
func (p *ConfigProtection) ValidateRoot(g *PathGuard) error {
	resolved, err := evalEffectivePath(g.root)
	if err != nil || !configPathEqual(resolved, g.effectiveRoot) {
		return ErrConfigurationAccess
	}
	info, err := PathIdentity(resolved)
	if err != nil || !os.SameFile(info, p.root) || p.validateRoot() != nil {
		return ErrConfigurationAccess
	}
	return nil
}

// Check protects the selected name and both the loaded and current file
// objects. A mutation additionally protects ancestors of the selected name.
// info is nil for an absent operand.
func (p *ConfigProtection) Check(path string, info, parent os.FileInfo, mutation bool) error {
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
