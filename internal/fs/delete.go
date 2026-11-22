package fs

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/resource"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Delete deletes a file or directory.
func (f *LocalFileSystem) Delete(path string, recursive bool, admission ...func() error) error {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(path); err != nil {
		return err
	}

	safePath, err := f.resolveExisting(path)
	if err != nil {
		return err
	}
	operand, err := f.inspectEffect(safePath.String(), true)
	if err != nil {
		return err
	}
	// Root-bound namespace APIs cannot remove their own root entry. Preserve
	// configuration denial above and reject before any child effects.
	if pathsEquivalent(safePath.String(), f.guard.Root()) {
		return security.ErrOutsideRoot
	}
	info := operand.info
	if err := f.checkOperandName(path, operand, false); err != nil {
		return err
	}
	if info == nil {
		return ErrNotFound
	}

	if info.IsDir() && !operand.leafLink {
		if recursive {
			return f.deleteProtectedTree(operand, admission)
		}

		dir, err := f.openNative(operand, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		entries, readErr := dir.ReadDir(1)
		_ = dir.Close()
		if readErr != nil && readErr != io.EOF {
			return readErr
		}

		if len(entries) > 0 {
			if int64(len(entries[0].Name())) > f.limits.MaxScanSize {
				return ErrLimitExceeded
			}
			return ErrDirectoryNotEmpty
		}
	}

	if err := f.recheckEffect(operand, true); err != nil {
		return err
	}
	if err := admitEffect(admission); err != nil {
		return err
	}
	return f.removeNative(operand)
}

type treeEffect struct {
	effectPath
	link os.FileInfo
}

// inspectTree checks all entries before changing any of them. WalkDir does
// not descend into symbolic links. A link whose resolved target is protected
// is rejected even though removing the link would leave its target intact.
func (f *LocalFileSystem) inspectTree(root effectPath, deleting bool, projected ...string) ([]treeEffect, error) {
	return f.inspectTreeWithMeter(root, deleting, nil, projected...)
}

func (f *LocalFileSystem) inspectTreeWithMeter(root effectPath, deleting bool, meter *resource.Meter, projected ...string) ([]treeEffect, error) {
	var entries []treeEffect
	// A move's inspection is bounded by scanned name bytes, not by the
	// deletion-specific entry ceiling. Every visited entry has a nonempty name.
	maxEntries := int(min(f.limits.MaxScanSize, int64(math.MaxInt)))
	if deleting {
		maxEntries = f.limits.MaxDeleteEntries
	}
	err := f.scanTreeWithMeter(root.path, maxEntries, func(path string, entry os.DirEntry) error {
		var err error
		if deleting && len(entries) >= f.limits.MaxDeleteEntries {
			return ErrLimitExceeded
		}
		var link os.FileInfo
		symlink := entry.Type()&os.ModeSymlink != 0
		if deleting && symlink {
			link, err = security.PathEntryIdentity(path)
			if err != nil || link.Mode()&os.ModeSymlink == 0 {
				return security.ErrConfigurationAccess
			}
		}
		operand, err := f.inspectStructuralEffect(path)
		if err != nil {
			return err
		}
		if operand.info == nil && !symlink {
			return security.ErrConfigurationAccess
		}
		if symlink {
			if f.fileTypes.Restricted() && operand.info == nil {
				return ErrFileTypeDenied
			}
			if len(projected) != 0 {
				link, err := security.RelativeLinkTarget(path)
				if err != nil || !filepath.IsAbs(link) {
					if !f.fileTypes.Restricted() {
						return security.ErrSymlinkDenied
					}
					return ErrFileTypeDenied
				}
			}
		}
		if len(projected) != 0 {
			relative, err := filepath.Rel(root.path, path)
			if err != nil {
				return ErrFileTypeDenied
			}
			if err := f.checkProjectedExclusions(root, projected[0], relative); err != nil {
				return err
			}
			if operand.info == nil || operand.info.IsDir() {
				return nil
			}
			projectedPath := effectPath{path: filepath.Join(projected[0], relative), info: operand.info}
			if symlink {
				projectedPath.effective = operand.effective
			}
			if err := f.checkFileType(projectedPath, true); err != nil {
				return err
			}
		}
		if deleting {
			entries = append(entries, treeEffect{effectPath: operand, link: link})
		}
		return nil
	}, meter)
	return entries, err
}

func (f *LocalFileSystem) deleteProtectedTree(root effectPath, admission []func() error) error {
	entries, err := f.inspectTree(root, true)
	if err != nil {
		return err
	}
	if err := f.recheckEffect(root, true); err != nil {
		return err
	}
	if err := admitEffect(admission); err != nil {
		return err
	}
	slices.Reverse(entries)
	for _, expected := range entries {
		// Children are reached through the authorized tree without
		// following directory links or interpreting client-supplied paths.
		if err := f.recheckTreeEntry(expected); err != nil {
			return err
		}
		current, err := f.inspectEffectContext(expected.path, true, expected.structural)
		if err != nil {
			return err
		}
		if err := f.removeNative(current); err != nil {
			return err
		}
	}
	if err := f.recheckEffect(root, true); err != nil {
		return err
	}
	return f.removeNative(root)
}

func (f *LocalFileSystem) recheckTreeEntry(expected treeEffect) error {
	// Reapply protection to the current resolved target. For an unlink, the
	// entry is the operand even if a prior removal made its target disappear.
	current, err := f.inspectEffectContext(expected.path, true, expected.structural)
	if err != nil || !sameEffectInfo(expected.entryParent, current.entryParent) {
		return security.ErrConfigurationAccess
	}
	if expected.link != nil {
		link, err := security.PathEntryIdentity(expected.path)
		if err != nil || !sameEffectInfo(expected.link, link) {
			return security.ErrConfigurationAccess
		}
	} else if !pathsEquivalent(expected.effective, current.effective) || !sameEffectInfo(expected.info, current.info) {
		return security.ErrConfigurationAccess
	}
	return nil
}
