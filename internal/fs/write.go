package fs

import (
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Write writes a file. Existing files are only overwritten when overwrite is true.
func (f *LocalFileSystem) Write(path string, content []byte, overwrite bool, admission ...func() error) error {
	_, err := f.WriteMeasured(path, content, overwrite, admission...)
	return err
}

// WriteMeasured also returns the byte count actually reported by native I/O.
func (f *LocalFileSystem) WriteMeasured(path string, content []byte, overwrite bool, admission ...func() error) (*uint64, error) {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(path); err != nil {
		return nil, err
	}

	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return nil, err
	}

	if int64(len(content)) > min(f.limits.MaxFileSize, f.limits.MaxWriteSize) {
		return nil, ErrLimitExceeded
	}

	target, err := f.inspectEffect(safePath.String(), true)
	if err != nil {
		return nil, err
	}
	if err := f.checkOperandName(path, target, true); err != nil {
		return nil, err
	}
	if info := target.info; info != nil {
		if info.IsDir() {
			return nil, ErrPathIsDirectory
		}

		if !overwrite {
			return nil, ErrFileExists
		}
	}

	file, err := f.openDestination(target, 0o600, func() error { return admitEffect(admission) })
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := f.checkDestination(file, target); err != nil {
		return nil, err
	}
	written, err := file.Write(content)
	observed := uint64(written)
	if err != nil {
		return &observed, err
	}

	if written != len(content) {
		return &observed, io.ErrShortWrite
	}

	return &observed, nil
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
		if err := f.mkdirNative(operand, 0o700); err != nil {
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
		operand, err := f.inspectPath(safePath.String())
		if err != nil {
			return false, err
		}
		if err := f.recheckPath(operand); err != nil {
			return false, err
		}
		return false, admitEffect(admission)
	}
	return f.mkdirProtected(safePath.String(), admission)
}
