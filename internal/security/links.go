package security

import (
	"os"
	"path/filepath"
	"strings"
)

func relativeLinkTarget(path string) (string, error) {
	target, err := nativeLinkTarget(path)
	if err != nil {
		return "", err
	}
	if target == "" || filepath.IsAbs(target) || filepath.VolumeName(target) != "" ||
		strings.HasPrefix(target, string(filepath.Separator)) || isUNCPath(target) {
		return "", ErrSymlinkDenied
	}
	return target, nil
}

// RelativeLinkTarget returns the native relative substitute target after
// platform classification. It reads link metadata only and grants no access.
func RelativeLinkTarget(path string) (string, error) { return relativeLinkTarget(path) }

// EffectivePath resolves existing components with the same native link
// semantics as PathGuard. Missing suffixes retain their exact names. This
// metadata helper is not authorization; callers must first use their guard.
func EffectivePath(path string) (string, error) {
	current := filepath.Clean(path)
	var missing []string
	for {
		effective, err := evalEffectivePath(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				effective = filepath.Join(effective, missing[i])
			}
			return filepath.Clean(effective), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// LinkIdentity is an opaque metadata-only snapshot. It grants no access.
// Native consumers recheck it together with their opened parent and object.
type LinkIdentity struct {
	path         string
	info         os.FileInfo
	target       string
	selectedPath string
}

// HasLeafLink identifies the input entry in an already captured link snapshot.
// It grants no access and does not classify or follow an uncaptured object.
func HasLeafLink(path string, links []LinkIdentity) bool {
	for _, link := range links {
		if configPathEqual(path, link.selectedPath) {
			return true
		}
	}
	return false
}

// LinkIdentities records every followed link, including chained targets.
// Callers first resolve the operand through PathGuard. Missing suffixes have
// no link identities; their existing parents are still inspected.
func (g *PathGuard) LinkIdentities(path string) ([]LinkIdentity, error) {
	if !isInsideRoot(g.root, path) {
		return nil, ErrSymlinkDenied
	}
	relative, err := filepath.Rel(g.root, path)
	if err != nil {
		return nil, ErrOutsideRoot
	}
	return walkLinkComponents(g.effectiveRoot, relative, path, nil, 0, true)
}

func snapshotLinks(base, path string) ([]LinkIdentity, error) {
	if !isInsideRoot(base, path) {
		return nil, ErrSymlinkDenied
	}
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return nil, ErrOutsideRoot
	}
	return walkLinkComponents(base, relative, path, nil, 0, true)
}

// walkLinkComponents shares the actual component order between policy checks
// and identity snapshots. A target is spliced before the remaining components;
// it is never cleaned across an unresolved link. current is therefore always
// the resolved parent when a relative target or its ".." component is applied.
func walkLinkComponents(base, relative, selected string, policy *Policy, depth int, capture bool) ([]LinkIdentity, error) {
	var links []LinkIdentity
	current := base
	pending := strings.Split(relative, string(filepath.Separator))
	selectingLeaf := true
	for len(pending) > 0 {
		if depth > 8 {
			return nil, ErrSymlinkDenied
		}
		part := pending[0]
		pending = pending[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			current = filepath.Dir(current)
			if !isInsideRoot(base, current) {
				return nil, ErrOutsideRoot
			}
			continue
		}
		if policy != nil && !policy.AllowHiddenFiles && isHiddenName(part) {
			return nil, ErrHiddenPathDenied
		}
		entry := filepath.Join(current, part)
		if policy != nil {
			if err := validatePathMetadata(entry, *policy); err != nil {
				return nil, err
			}
		}
		var info os.FileInfo
		var err error
		if capture {
			info, err = PathEntryIdentity(entry)
		} else {
			info, err = os.Lstat(entry)
		}
		if os.IsNotExist(err) && policy == nil {
			return links, nil
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := relativeLinkTarget(entry)
			if err != nil {
				return nil, err
			}
			leaf := len(pending) == 0 && selectingLeaf
			if capture {
				link := LinkIdentity{path: entry, info: info, target: target}
				if leaf {
					link.selectedPath = selected
				}
				links = append(links, link)
			}
			if leaf {
				selectingLeaf = false
			}
			depth++
			pending = append(strings.Split(filepath.FromSlash(target), string(filepath.Separator)), pending...)
			continue
		}
		if len(pending) > 0 && !info.IsDir() {
			return nil, ErrRootNotDirectory
		}
		current = entry
	}
	if depth > 8 {
		return nil, ErrSymlinkDenied
	}
	return links, nil
}

// Root ancestors participate in native resolution too. Inspect their reparse
// classification without treating an explicitly configured ancestor's hidden
// name as a client selection. Only relative supported links may be followed.
func validateRootChain(path string, policy Policy, depth int) error {
	policy.AllowHiddenFiles = true
	base := filepath.VolumeName(path) + string(filepath.Separator)
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return ErrOutsideRoot
	}
	_, err = walkLinkComponents(base, relative, path, &policy, depth, false)
	return err
}

// SameLinks reports unchanged order, spelling, link objects and raw targets.
func SameLinks(left, right []LinkIdentity) bool {
	if len(left) != len(right) {
		return false
	}
	for i, l := range left {
		r := right[i]
		if !configPathEqual(l.path, r.path) || l.selectedPath != r.selectedPath || l.target != r.target || !os.SameFile(l.info, r.info) || !SamePolicyMetadata(l.info, r.info) {
			return false
		}
	}
	return true
}

// SamePolicyMetadata compares mutable native policy metadata only. It is not
// an object-identity or authorization check; missing observations never match.
func SamePolicyMetadata(left, right os.FileInfo) bool {
	if left == nil || right == nil {
		return false
	}
	return samePolicyMetadata(left, right)
}
