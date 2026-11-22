package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// ErrPathExcluded contains no identifying path, root or pattern.
var ErrPathExcluded = fmt.Errorf("path access denied: %w", os.ErrPermission)

const (
	maxExclusionRouteBytes = 4096
	maxExclusionRouteDepth = 128
	maxExclusionMatchWork  = 1 << 16
)

type exclusionRoot struct {
	guard  *security.PathGuard
	volume string
	names  []string
	rules  config.PathExclusions
}

// All backends share the same immutable policy bindings and effect lock.
// Roots with empty policies do not contribute a rule.
type exclusionRoots []exclusionRoot

type exclusionRoute struct {
	volume  string
	names   []string
	raw     []string
	parents []string
}

type exclusionChecks struct {
	volumes map[string]string
	cases   map[string]bool
	entries map[string]string
	work    int
}

func newExclusionChecks() exclusionChecks {
	return exclusionChecks{
		volumes: make(map[string]string), cases: make(map[string]bool),
		entries: make(map[string]string), work: maxExclusionMatchWork,
	}
}

func bindExclusionRoots(filesystems []*LocalFileSystem, rules []config.PathExclusions) error {
	if len(rules) == 0 {
		return nil
	}
	if len(rules) != len(filesystems) {
		return security.ErrProtectedPath
	}
	if !slices.ContainsFunc(rules, config.PathExclusions.Active) {
		return nil
	}
	roots := make(exclusionRoots, 0, len(rules))
	checks := newExclusionChecks()
	for i, f := range filesystems {
		route, err := checks.rootRoute(f.guard)
		if err != nil {
			return security.ErrProtectedPath
		}
		if rules[i].Active() {
			roots = append(roots, exclusionRoot{f.guard, route.volume, route.names, rules[i]})
		}
	}
	for _, f := range filesystems {
		f.exclusions = roots
	}
	return nil
}

func (c *exclusionChecks) rootRoute(guard *security.PathGuard) (exclusionRoute, error) {
	path := guard.EffectiveRoot()
	info, err := security.PathIdentity(path)
	if err != nil || guard.ValidateRootIdentity(info) != nil {
		return exclusionRoute{}, security.ErrConfigurationAccess
	}
	actual, err := security.PathIdentity(guard.Root())
	if err != nil || !os.SameFile(info, actual) {
		return exclusionRoute{}, security.ErrConfigurationAccess
	}
	route, err := c.route(path)
	if err != nil {
		return exclusionRoute{}, err
	}
	// Include the root itself in the native proof even at the volume root.
	if _, err := c.directoryMode(path); err != nil {
		return exclusionRoute{}, err
	}
	return route, nil
}

func (c *exclusionChecks) route(path string) (exclusionRoute, error) {
	if !filepath.IsAbs(path) || len(path) > maxExclusionRouteBytes {
		return exclusionRoute{}, os.ErrPermission
	}
	drive := filepath.VolumeName(path) + string(filepath.Separator)
	start := len(drive)
	volume, ok := c.volumes[drive]
	if !ok {
		var err error
		volume, err = nativeExclusionVolume(drive)
		if err != nil {
			return exclusionRoute{}, err
		}
		c.volumes[drive] = volume
	}
	route := exclusionRoute{volume: volume}
	if start == len(path) {
		return route, nil
	}
	route.raw = strings.Split(path[start:], string(filepath.Separator))
	if len(route.raw) > maxExclusionRouteDepth {
		return exclusionRoute{}, os.ErrPermission
	}
	route.names = make([]string, 0, len(route.raw))
	route.parents = make([]string, 0, len(route.raw))
	parent := drive
	for _, raw := range route.raw {
		if raw == "" || raw == "." || raw == ".." || !utf8.ValidString(raw) || ambiguousNativeFileName(raw) {
			return exclusionRoute{}, os.ErrPermission
		}
		route.parents = append(route.parents, parent)
		if exclusionCaseNeedsTopologyCheck {
			// Casefold or a mount anywhere in the path could also change
			// which root applies, so classify even outside a matching root.
			if _, err := c.directoryMode(parent); err != nil && !errors.Is(err, os.ErrNotExist) {
				return exclusionRoute{}, err
			}
		}
		parent = filepath.Join(parent, raw)
		name, known := c.entries[parent]
		if !known {
			var err error
			name, err = nativeExclusionName(parent)
			if errors.Is(err, os.ErrNotExist) {
				name = raw
			} else if err != nil {
				return exclusionRoute{}, err
			}
			c.entries[parent] = name
		}
		if name == "" || !utf8.ValidString(name) || ambiguousNativeFileName(name) {
			return exclusionRoute{}, os.ErrPermission
		}
		route.names = append(route.names, name)
	}
	return route, nil
}

func (c *exclusionChecks) directoryMode(path string) (bool, error) {
	if mode, ok := c.cases[path]; ok {
		return mode, nil
	}
	mode, err := nativeExclusionDirectory(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) && exclusionCaseNeedsTopologyCheck {
		// A lexical route can include links already classified by PathGuard.
		// Reuse its resolver and require the resolved directory's native proof.
		if resolved, resolveErr := security.EffectivePath(path); resolveErr == nil && resolved != path {
			mode, err = nativeExclusionDirectory(resolved)
		}
	}
	if err == nil {
		c.cases[path] = mode
	}
	return mode, err
}

func (c *exclusionChecks) verify(roots exclusionRoots) error {
	for _, root := range roots {
		actual, err := c.rootRoute(root.guard)
		if err != nil || actual.volume != root.volume || !slices.Equal(actual.names, root.names) {
			return security.ErrConfigurationAccess
		}
	}
	return nil
}

type exclusionProjection struct{ from, to string }

func (c *exclusionChecks) check(roots exclusionRoots, path string, projected exclusionProjection) error {
	route, err := c.route(path)
	if err != nil {
		return security.ErrConfigurationAccess
	}
	for _, root := range roots {
		n := len(root.names)
		if route.volume != root.volume || len(route.names) <= n || !slices.Equal(root.names, route.names[:n]) {
			continue
		}
		names := make([]config.ExclusionName, 0, len(route.names)-n)
		spelled := make([]config.ExclusionName, 0, len(route.names)-n)
		for i, name := range route.names[n:] {
			parent := route.parents[n+i]
			if projected.to != "" {
				rel, err := filepath.Rel(projected.to, parent)
				if err == nil && filepath.IsLocal(rel) {
					// The directory being moved retains its native case mode.
					parent = filepath.Join(projected.from, rel)
				}
			}
			fold, err := c.directoryMode(parent)
			if errors.Is(err, os.ErrNotExist) && projected.to == "" {
				// Future Mkdir parents in the ext mount inherit an exact
				// namespace. On Windows reject a potential folded match
				// until the newly created directory can be classified.
				fold = !exclusionCaseNeedsTopologyCheck
			} else if err != nil {
				return security.ErrConfigurationAccess
			}
			names = append(names, config.ExclusionName{Text: name, CaseInsensitive: fold})
			spelled = append(spelled, config.ExclusionName{Text: route.raw[n+i], CaseInsensitive: fold})
		}
		for _, components := range [][]config.ExclusionName{names, spelled} {
			denied, err := root.rules.Excludes(components, &c.work)
			if err != nil {
				return security.ErrConfigurationAccess
			}
			if denied {
				return ErrPathExcluded
			}
			if slices.Equal(names, spelled) {
				break
			}
		}
	}
	return nil
}

func (f *LocalFileSystem) inspectExclusionLinks(path string) ([]security.LinkIdentity, error) {
	if len(f.exclusions) == 0 {
		return f.guard.LinkIdentities(path)
	}
	checks := newExclusionChecks()
	if err := checks.verify(f.exclusions); err != nil {
		return nil, err
	}
	selected, err := filepath.Rel(f.guard.Root(), path)
	if err != nil || !filepath.IsLocal(selected) {
		return nil, security.ErrConfigurationAccess
	}
	links, paths, err := f.guard.ExclusionTraversal(path)
	if err != nil {
		return nil, security.ErrConfigurationAccess
	}
	paths = append(paths, path, filepath.Join(f.guard.EffectiveRoot(), selected))
	seen := make(map[string]bool, len(paths))
	for _, route := range paths {
		if seen[route] {
			continue
		}
		seen[route] = true
		if err := checks.check(f.exclusions, route, exclusionProjection{}); err != nil {
			return nil, err
		}
	}
	return links, nil
}

func (f *LocalFileSystem) resolveExisting(path string) (security.SafePath, error) {
	resolved, err := f.guard.ResolveExisting(path)
	if len(f.exclusions) == 0 || !errors.Is(err, os.ErrNotExist) {
		return resolved, err
	}
	// Resolve the missing suffix as an operand before returning not found.
	missing, createErr := f.guard.ResolveForCreate(path)
	if createErr != nil {
		return resolved, createErr
	}
	if _, excluded := f.inspectExclusionLinks(missing.String()); excluded != nil {
		return resolved, excluded
	}
	return resolved, err
}

func (f *LocalFileSystem) checkProjectedExclusions(source effectPath, target string, relative string) error {
	if len(f.exclusions) == 0 {
		return nil
	}
	checks := newExclusionChecks()
	if err := checks.verify(f.exclusions); err != nil {
		return err
	}
	effective, err := security.EffectivePath(target)
	if err != nil {
		return security.ErrConfigurationAccess
	}
	for _, route := range []string{target, effective} {
		if err := checks.check(f.exclusions, filepath.Join(route, relative), exclusionProjection{from: source.effective, to: route}); err != nil {
			return err
		}
		if effective == target {
			break
		}
	}
	return nil
}
