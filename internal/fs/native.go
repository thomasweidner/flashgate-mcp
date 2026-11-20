package fs

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func (f *LocalFileSystem) validateNativeRoot() error {
	bound, err := f.bindNative(f.guard.Root(), false)
	if err != nil {
		return err
	}
	defer bound.close()
	info, err := bound.info()
	if err != nil || info == nil || !info.IsDir() {
		return security.ErrOutsideRoot
	}
	return nil
}

// bindNative is an operation-scoped binding, not a policy or root registry.
// PathGuard owns resolution. Native backends receive only its authorized
// effective names and bind the parent before opening or changing the leaf.
func (f *LocalFileSystem) bindNative(path string, entry bool) (*nativePath, error) {
	if f.protection != nil {
		if err := f.protection.ValidateRoot(); err != nil {
			return nil, err
		}
	}
	if _, err := f.guard.RevalidateEffect(path); err != nil {
		return nil, err
	}
	effective, err := effectivePathForComparison(path)
	if entry {
		var parent string
		parent, err = security.EffectivePath(filepath.Dir(path))
		effective = filepath.Join(parent, filepath.Base(path))
	}
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(f.guard.EffectiveRoot(), effective)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, security.ErrOutsideRoot
	}
	root, err := os.OpenRoot(f.guard.EffectiveRoot())
	if err != nil {
		return nil, err
	}
	probe, err := root.Open(".")
	if err == nil {
		var info os.FileInfo
		info, err = probe.Stat()
		if err == nil {
			err = f.guard.ValidateRootIdentity(info)
		}
		_ = probe.Close()
	}
	if err != nil {
		_ = root.Close()
		if f.protection != nil {
			return nil, security.ErrConfigurationAccess
		}
		return nil, security.ErrOutsideRoot
	}
	bound, err := bindNativePath(root, relative, effective)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return bound, nil
}

func (f *LocalFileSystem) interceptNative(stage string) {
	if f.beforeNative != nil {
		f.beforeNative(stage)
	}
}

func (f *LocalFileSystem) openNative(expected effectPath, flag int, mode os.FileMode, checks ...func() error) (*os.File, error) {
	bound, err := f.bindNative(expected.path, false)
	if err != nil {
		return nil, err
	}
	defer bound.close()
	f.interceptNative("open")
	for _, check := range checks {
		if check != nil {
			if err := check(); err != nil {
				return nil, err
			}
		}
	}
	if err := f.recheckPath(expected); err != nil {
		return nil, err
	}
	if err := bound.matches(expected.parent, expected.info); err != nil {
		return nil, err
	}
	return bound.open(flag, mode)
}

func (f *LocalFileSystem) removeNative(expected effectPath) error {
	bound, err := f.bindNative(expected.path, true)
	if err != nil {
		return err
	}
	defer bound.close()
	entry, err := bound.info()
	if err != nil || entry == nil {
		return security.ErrConfigurationAccess
	}
	f.interceptNative("remove")
	if err := f.recheckEffect(expected, true); err != nil {
		return err
	}
	// For a followed leaf link, the target was checked above. The namespace
	// effect removes the link entry itself and never its target directory.
	if err := bound.matches(expected.entryParent, entry); err != nil {
		return err
	}
	return bound.remove(entry.IsDir())
}

func (f *LocalFileSystem) mkdirNative(expected effectPath, mode os.FileMode) error {
	bound, err := f.bindNative(expected.path, true)
	if err != nil {
		return err
	}
	defer bound.close()
	f.interceptNative("mkdir")
	if err := f.recheckEffect(expected, true); err != nil {
		return err
	}
	if err := bound.matches(expected.entryParent, nil); err != nil {
		return err
	}
	return bound.mkdir(mode)
}

func (f *LocalFileSystem) renameNative(source, target effectPath, checks ...func() error) error {
	left, err := f.bindNative(source.path, true)
	if err != nil {
		return err
	}
	defer left.close()
	right, err := f.bindNative(target.path, true)
	if err != nil {
		return err
	}
	defer right.close()
	leftEntry, err := left.info()
	if err != nil {
		return err
	}
	rightEntry, err := right.info()
	if err != nil {
		return err
	}
	f.interceptNative("rename")
	if err := f.recheckEffect(source, true); err != nil {
		return err
	}
	if err := f.recheckEffect(target, true); err != nil {
		return err
	}
	for _, check := range checks {
		if check != nil {
			if err := check(); err != nil {
				return err
			}
		}
	}
	if err := f.recheckEffect(source, true); err != nil {
		return err
	}
	if err := f.recheckEffect(target, true); err != nil {
		return err
	}
	if err := left.matches(source.entryParent, leftEntry); err != nil {
		return err
	}
	if err := right.matches(target.entryParent, rightEntry); err != nil {
		return err
	}
	return left.rename(right)
}

func nativeInfo(file *os.File, err error) (os.FileInfo, error) {
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}
