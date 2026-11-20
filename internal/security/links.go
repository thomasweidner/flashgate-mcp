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
	path   string
	info   os.FileInfo
	target string
}

// HasLeafLink identifies the input entry in an already captured link snapshot.
// It grants no access and does not classify or follow an uncaptured object.
func HasLeafLink(path string, links []LinkIdentity) bool {
	for _, link := range links {
		if configPathEqual(path, link.path) {
			return true
		}
	}
	return false
}

// LinkIdentities records every followed link, including chained targets.
// Callers first resolve the operand through PathGuard. Missing suffixes have
// no link identities; their existing parents are still inspected.
func (g *PathGuard) LinkIdentities(path string) ([]LinkIdentity, error) {
	return snapshotLinks(g.root, path)
}

func snapshotLinks(base, path string) ([]LinkIdentity, error) {
	var links []LinkIdentity
	var walk func(string, int) error
	walk = func(path string, depth int) error {
		if depth > 8 || !isInsideRoot(base, path) {
			return ErrSymlinkDenied
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return ErrOutsideRoot
		}
		if relative == "." {
			return nil
		}
		current := base
		for _, part := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, part)
			info, err := PathEntryIdentity(current)
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := relativeLinkTarget(current)
			if err != nil {
				return err
			}
			links = append(links, LinkIdentity{current, info, target})
			if err := walk(filepath.Join(filepath.Dir(current), target), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	err := walk(path, 0)
	return links, err
}

// Root ancestors participate in native resolution too. Inspect their reparse
// classification without treating an explicitly configured ancestor's hidden
// name as a client selection. Only relative supported links may be followed.
func validateRootChain(path string, policy Policy, depth int) error {
	if depth > 8 {
		return ErrSymlinkDenied
	}
	policy.AllowHiddenFiles = true
	base := filepath.VolumeName(path) + string(filepath.Separator)
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return ErrOutsideRoot
	}
	current := base
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if err := validatePathMetadata(current, policy); err != nil {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := relativeLinkTarget(current)
			if err != nil {
				return err
			}
			if err := validateRootChain(filepath.Join(filepath.Dir(current), target), policy, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// SameLinks reports unchanged order, spelling, link objects and raw targets.
func SameLinks(left, right []LinkIdentity) bool {
	if len(left) != len(right) {
		return false
	}
	for i, l := range left {
		r := right[i]
		if !configPathEqual(l.path, r.path) || l.target != r.target || !os.SameFile(l.info, r.info) || !SamePolicyMetadata(l.info, r.info) {
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
