package fs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Write writes a file. Existing files are only overwritten when overwrite is true.
func (f *LocalFileSystem) Write(path string, content []byte, overwrite bool, admission ...func() error) error {
	f.effects.Lock()
	defer f.effects.Unlock()

	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return err
	}

	if int64(len(content)) > min(f.limits.MaxFileSize, f.limits.MaxWriteSize) {
		return ErrLimitExceeded
	}

	target, err := f.inspectEffect(safePath.String(), true)
	if err != nil {
		return err
	}
	if info := target.info; info != nil {
		if info.IsDir() {
			return ErrPathIsDirectory
		}

		if !overwrite {
			return ErrFileExists
		}
	}

	file, err := f.openDestination(target, 0o600, func() error { return admitEffect(admission) })
	if err != nil {
		return err
	}
	defer file.Close()

	if err := f.checkDestination(file, target); err != nil {
		return err
	}
	written, err := file.Write(content)
	if err != nil {
		return err
	}

	if written != len(content) {
		return io.ErrShortWrite
	}

	return nil
}

func (f *LocalFileSystem) mkdirProtected(path string, admission []func() error) (bool, error) {
	var missing []string
	var parentInfo os.FileInfo
	for current := path; ; current = filepath.Dir(current) {
		info, err := effectInfo(current)
		if err != nil {
			return false, err
		}
		if info != nil {
			if current == path {
				if _, err := f.inspectEffect(current, true); err != nil {
					return false, err
				}
			}
			if !info.IsDir() {
				return false, ErrPathIsNotDirectory
			}
			parentInfo = info
			break
		}
		if _, err := f.inspectEffect(current, true); err != nil {
			return false, err
		}
		missing = append(missing, current)
		if filepath.Dir(current) == current {
			return false, security.ErrConfigurationAccess
		}
	}
	if len(missing) == 0 {
		return false, admitEffect(admission)
	}
	slices.Reverse(missing)
	for index, current := range missing {
		operand, err := f.inspectEffect(current, true)
		if err != nil || operand.info != nil || !sameEffectInfo(operand.parent, parentInfo) {
			return false, security.ErrConfigurationAccess
		}
		if err := f.recheckEffect(operand, true); err != nil {
			return false, err
		}
		if err := admitEffect(admission); err != nil {
			return false, err
		}
		if err := os.Mkdir(current, 0o700); err != nil {
			return false, err
		}
		if index < len(missing)-1 {
			parentInfo, err = effectInfo(current)
			if err != nil || parentInfo == nil || !parentInfo.IsDir() {
				return false, security.ErrConfigurationAccess
			}
		}
	}
	return true, nil
}

// Mkdir creates a directory and any missing parent directories and reports whether the leaf was created.
func (f *LocalFileSystem) Mkdir(path string, admission ...func() error) (bool, error) {
	f.effects.Lock()
	defer f.effects.Unlock()

	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return false, err
	}
	if pathsEquivalent(safePath.String(), f.guard.Root()) {
		return false, admitEffect(admission)
	}
	if f.protection != nil {
		return f.mkdirProtected(safePath.String(), admission)
	}
	if err := admitEffect(admission); err != nil {
		return false, err
	}

	parent := safePath.Dir()
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return false, err
	}

	if err := os.Mkdir(safePath.String(), 0o700); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrExist) {
		return false, err
	}

	info, err := os.Stat(safePath.String())
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, ErrPathIsNotDirectory
	}

	return false, nil
}
