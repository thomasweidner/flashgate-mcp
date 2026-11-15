package fs

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Delete deletes a file or directory.
func (f *LocalFileSystem) Delete(path string, recursive bool) error {
	f.effects.Lock()
	defer f.effects.Unlock()

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return err
	}

	operand, err := f.inspectEffect(safePath.String(), true)
	if err != nil {
		return err
	}
	info := operand.info
	if info == nil {
		return ErrNotFound
	}

	if info.IsDir() {
		if recursive {
			if f.protection != nil {
				return f.deleteProtectedTree(operand)
			}
			if err := f.ensureRecursiveDeleteWithinLimit(safePath.String()); err != nil {
				return err
			}

			return os.RemoveAll(safePath.String())
		}

		entries, err := os.ReadDir(safePath.String())
		if err != nil {
			return err
		}

		if len(entries) > 0 {
			return ErrDirectoryNotEmpty
		}
	}

	if err := f.recheckEffect(operand, true); err != nil {
		return err
	}
	return os.Remove(safePath.String())
}

// inspectTree checks all entries before changing any of them. WalkDir does
// not descend into symbolic links. A link whose resolved target is protected
// is rejected even though removing the link would leave its target intact.
func (f *LocalFileSystem) inspectTree(root effectPath, deleting bool) ([]effectPath, error) {
	var entries []effectPath
	err := filepath.WalkDir(root.path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root.path {
			return nil
		}
		if deleting && len(entries) >= f.limits.MaxDeleteEntries {
			return ErrLimitExceeded
		}
		operand, err := f.inspectEffect(path, true)
		if err != nil {
			return err
		}
		if operand.info == nil && entry.Type()&os.ModeSymlink == 0 {
			return security.ErrConfigurationAccess
		}
		if deleting {
			entries = append(entries, operand)
		}
		return nil
	})
	return entries, err
}

func (f *LocalFileSystem) deleteProtectedTree(root effectPath) error {
	entries, err := f.inspectTree(root, true)
	if err != nil {
		return err
	}
	if err := f.recheckEffect(root, true); err != nil {
		return err
	}
	slices.Reverse(entries)
	for _, expected := range entries {
		// Children are reached through the authorized tree without
		// following directory links or interpreting client-supplied paths.
		current, err := f.inspectEffect(expected.path, true)
		if err != nil || !pathsEquivalent(expected.effective, current.effective) ||
			!sameEffectInfo(expected.info, current.info) || !sameEffectInfo(expected.parent, current.parent) {
			return security.ErrConfigurationAccess
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

func (f *LocalFileSystem) ensureRecursiveDeleteWithinLimit(root string) error {
	entries := 0

	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == root {
			return nil
		}

		entries++
		if entries > f.limits.MaxDeleteEntries {
			return ErrLimitExceeded
		}

		return nil
	})
}
