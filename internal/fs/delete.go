package fs

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Delete deletes a file or directory.
func (f *LocalFileSystem) Delete(path string, recursive bool, admission ...func() error) error {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(path); err != nil {
		return err
	}

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return err
	}

	operand, err := f.inspectEffect(safePath.String(), true)
	if err != nil {
		return err
	}
	info := operand.info
	if err := f.checkOperandName(path, operand, false); err != nil {
		return err
	}
	if info == nil {
		return ErrNotFound
	}

	if info.IsDir() {
		if recursive {
			if f.checksIdentity() {
				return f.deleteProtectedTree(operand, admission)
			}
			if err := f.ensureRecursiveDeleteWithinLimit(safePath.String()); err != nil {
				return err
			}

			if err := admitEffect(admission); err != nil {
				return err
			}
			return os.RemoveAll(safePath.String())
		}

		dir, err := os.Open(safePath.String())
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
	return os.Remove(safePath.String())
}

type treeEffect struct {
	effectPath
	link os.FileInfo
}

// inspectTree checks all entries before changing any of them. WalkDir does
// not descend into symbolic links. A link whose resolved target is protected
// is rejected even though removing the link would leave its target intact.
func (f *LocalFileSystem) inspectTree(root effectPath, deleting bool, projected ...string) ([]treeEffect, error) {
	var entries []treeEffect
	// A move's inspection is bounded by scanned name bytes, not by the
	// deletion-specific entry ceiling. Every visited entry has a nonempty name.
	maxEntries := int(min(f.limits.MaxScanSize, int64(math.MaxInt)))
	if deleting {
		maxEntries = f.limits.MaxDeleteEntries
	}
	err := f.scanTree(root.path, maxEntries, func(path string, entry os.DirEntry) error {
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
		operand, err := f.inspectEffect(path, true)
		if err != nil {
			return err
		}
		if operand.info == nil && !symlink {
			return security.ErrConfigurationAccess
		}
		if f.fileTypes.Restricted() && symlink {
			if operand.info == nil {
				return ErrFileTypeDenied
			}
			if len(projected) != 0 {
				link, err := os.Readlink(path)
				if err != nil || !filepath.IsAbs(link) {
					return ErrFileTypeDenied
				}
			}
		}
		if len(projected) != 0 && operand.info != nil && !operand.info.IsDir() {
			relative, err := filepath.Rel(root.path, path)
			if err != nil {
				return ErrFileTypeDenied
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
	})
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
		if err := os.Remove(expected.path); err != nil {
			return err
		}
	}
	if err := f.recheckEffect(root, true); err != nil {
		return err
	}
	return os.Remove(root.path)
}

func (f *LocalFileSystem) recheckTreeEntry(expected treeEffect) error {
	// Reapply protection to the current resolved target. For an unlink, the
	// entry is the operand even if a prior removal made its target disappear.
	current, err := f.inspectEffect(expected.path, true)
	if err != nil || !sameEffectInfo(expected.parent, current.parent) {
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

func (f *LocalFileSystem) ensureRecursiveDeleteWithinLimit(root string) error {
	return f.scanTree(root, f.limits.MaxDeleteEntries, func(string, os.DirEntry) error { return nil })
}
