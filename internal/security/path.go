package security

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrEmptyRoot is returned when the configured root path is empty.
	ErrEmptyRoot = errors.New("root path must not be empty")

	// ErrRootNotDirectory is returned when the configured or effective root is not a directory.
	ErrRootNotDirectory = errors.New("root path is not a directory")

	// ErrAbsolutePath is returned when a user path is absolute.
	ErrAbsolutePath = errors.New("absolute paths are not allowed")

	// ErrPathTraversal is returned when a user path attempts to escape the root.
	ErrPathTraversal = errors.New("path traversal detected")

	// ErrOutsideRoot is returned when the resolved path is outside the configured root.
	ErrOutsideRoot = errors.New("access outside root denied")

	// ErrHiddenPathDenied is returned when hidden path access is disabled.
	ErrHiddenPathDenied = errors.New("hidden path access denied")

	// ErrUNCPathDenied is returned when UNC path access is disabled.
	ErrUNCPathDenied = errors.New("UNC path access denied")

	// ErrSymlinkDenied is returned when symlink following is disabled.
	ErrSymlinkDenied = errors.New("symlink access denied")

	// ErrReparsePointDenied is returned when Windows reparse point access is denied.
	ErrReparsePointDenied = errors.New("reparse point access denied")
)

// Policy controls optional filesystem access behavior.
type Policy struct {
	AllowHiddenFiles bool
	AllowUNCPaths    bool
	FollowSymlinks   bool
}

// DefaultPolicy returns the secure default filesystem policy.
func DefaultPolicy() Policy {
	return Policy{
		AllowHiddenFiles: false,
		AllowUNCPaths:    false,
		FollowSymlinks:   false,
	}
}

// SafePath represents a filesystem path that has passed validation.
type SafePath struct {
	path string
}

// String returns the absolute safe path.
func (p SafePath) String() string {
	return p.path
}

// Base returns the last path element.
func (p SafePath) Base() string {
	return filepath.Base(p.path)
}

// Dir returns the directory component.
func (p SafePath) Dir() string {
	return filepath.Dir(p.path)
}

// PathGuard validates and resolves user-provided paths against a sandbox root.
type PathGuard struct {
	root          string
	effectiveRoot string
	policy        Policy
}

// NewPathGuard creates a new PathGuard.
func NewPathGuard(root string) (*PathGuard, error) {
	return NewPathGuardWithPolicy(root, DefaultPolicy())
}

// NewPathGuardWithPolicy creates a new PathGuard with an explicit policy.
func NewPathGuardWithPolicy(root string, policy Policy) (*PathGuard, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrEmptyRoot
	}

	if !policy.AllowUNCPaths && isUNCPath(root) {
		return nil, ErrUNCPathDenied
	}

	absoluteRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}

	if !policy.AllowUNCPaths && isUNCPath(absoluteRoot) {
		return nil, ErrUNCPathDenied
	}

	if err := validatePathMetadata(absoluteRoot, policy); err != nil {
		return nil, err
	}

	effectiveRoot, err := evalEffectivePath(absoluteRoot)
	if err != nil {
		return nil, err
	}

	effectiveInfo, err := os.Stat(effectiveRoot)
	if err != nil {
		return nil, err
	}
	if !effectiveInfo.IsDir() {
		return nil, ErrRootNotDirectory
	}

	return &PathGuard{
		root:          absoluteRoot,
		effectiveRoot: effectiveRoot,
		policy:        policy,
	}, nil
}

// Root returns the absolute sandbox root.
func (g *PathGuard) Root() string {
	return g.root
}

// Policy returns the immutable policy enforced by this guard.
func (g *PathGuard) Policy() Policy {
	return g.policy
}

// Resolve validates and resolves a user path against the sandbox root.
func (g *PathGuard) Resolve(userPath string) (SafePath, error) {
	return g.ResolveForCreate(userPath)
}

// ResolveExisting validates and resolves an existing user path against the sandbox root.
func (g *PathGuard) ResolveExisting(userPath string) (SafePath, error) {
	resolvedPath, err := g.resolveLexical(userPath)
	if err != nil {
		return SafePath{}, err
	}

	if err := g.validateExistingPathPolicy(resolvedPath); err != nil {
		return SafePath{}, err
	}

	effectivePath, err := evalEffectivePath(resolvedPath)
	if err != nil {
		return SafePath{}, err
	}

	if !isInsideRoot(g.effectiveRoot, effectivePath) {
		return SafePath{}, ErrOutsideRoot
	}

	return SafePath{
		path: resolvedPath,
	}, nil
}

// ResolveForCreate validates a user path for creation or replacement.
func (g *PathGuard) ResolveForCreate(userPath string) (SafePath, error) {
	resolvedPath, err := g.resolveLexical(userPath)
	if err != nil {
		return SafePath{}, err
	}

	effectivePath, err := evalEffectivePath(resolvedPath)
	if err == nil {
		if err := g.validateExistingPathPolicy(resolvedPath); err != nil {
			return SafePath{}, err
		}

		if !isInsideRoot(g.effectiveRoot, effectivePath) {
			return SafePath{}, ErrOutsideRoot
		}

		return SafePath{
			path: resolvedPath,
		}, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return SafePath{}, err
	}

	effectiveParent, err := g.resolveExistingParent(resolvedPath)
	if err != nil {
		return SafePath{}, err
	}

	if !isInsideRoot(g.effectiveRoot, effectiveParent) {
		return SafePath{}, ErrOutsideRoot
	}

	return SafePath{
		path: resolvedPath,
	}, nil
}

func (g *PathGuard) resolveLexical(userPath string) (string, error) {
	normalizedUserPath := normalizeUserPath(userPath)

	if !g.policy.AllowUNCPaths && isUNCPath(normalizedUserPath) {
		return "", ErrUNCPathDenied
	}

	if filepath.IsAbs(normalizedUserPath) {
		return "", ErrAbsolutePath
	}

	if isTraversalPath(normalizedUserPath) {
		return "", ErrPathTraversal
	}

	if !g.policy.AllowHiddenFiles && hasHiddenPathComponent(normalizedUserPath) {
		return "", ErrHiddenPathDenied
	}

	resolvedPath, err := filepath.Abs(filepath.Join(g.root, normalizedUserPath))
	if err != nil {
		return "", err
	}

	if !isInsideRoot(g.root, resolvedPath) {
		return "", ErrOutsideRoot
	}

	return resolvedPath, nil
}

func (g *PathGuard) resolveExistingParent(path string) (string, error) {
	current := filepath.Clean(path)

	for {
		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrOutsideRoot
		}

		if !isInsideRoot(g.root, parent) {
			return "", ErrOutsideRoot
		}

		if err := g.validateExistingPathPolicy(parent); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				current = parent
				continue
			}

			return "", err
		}

		effectiveParent, err := evalEffectivePath(parent)
		if err == nil {
			return effectiveParent, nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		current = parent
	}
}

// AllowListEntry reports whether a directory entry may be exposed by list_directory.
func (g *PathGuard) AllowListEntry(parent SafePath, name string) (bool, error) {
	if !g.policy.AllowHiddenFiles && isHiddenName(name) {
		return false, nil
	}

	entryPath := filepath.Join(parent.String(), name)
	if !isInsideRoot(g.root, entryPath) {
		return false, ErrOutsideRoot
	}

	if err := validatePathMetadata(entryPath, g.policy); err != nil {
		if errors.Is(err, ErrHiddenPathDenied) ||
			errors.Is(err, ErrSymlinkDenied) ||
			errors.Is(err, ErrReparsePointDenied) {
			return false, nil
		}

		return false, err
	}

	effectivePath, err := evalEffectivePath(entryPath)
	if err != nil {
		return false, err
	}

	if !isInsideRoot(g.effectiveRoot, effectivePath) {
		return false, nil
	}

	return true, nil
}

func (g *PathGuard) validateExistingPathPolicy(path string) error {
	relative, err := filepath.Rel(g.root, path)
	if err != nil {
		return ErrOutsideRoot
	}

	if relative == "." {
		return validatePathMetadata(g.root, g.policy)
	}

	current := g.root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}

		if !g.policy.AllowHiddenFiles && isHiddenName(component) {
			return ErrHiddenPathDenied
		}

		current = filepath.Join(current, component)
		if err := validatePathMetadata(current, g.policy); err != nil {
			return err
		}
	}

	return nil
}

func normalizeUserPath(userPath string) string {
	if strings.TrimSpace(userPath) == "" {
		return "."
	}

	return filepath.Clean(userPath)
}

func isTraversalPath(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func hasHiddenPathComponent(path string) bool {
	if path == "." {
		return false
	}

	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if isHiddenName(component) {
			return true
		}
	}

	return false
}

func isHiddenName(name string) bool {
	return name != "" && name != "." && name != ".." && strings.HasPrefix(name, ".")
}

func isInsideRoot(root string, resolvedPath string) bool {
	relative, err := filepath.Rel(root, resolvedPath)
	if err != nil {
		return false
	}

	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
