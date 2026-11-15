package fs

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type effectPath struct {
	path      string
	info      os.FileInfo
	parent    os.FileInfo
	effective string
}

// inspectEffect takes a metadata snapshot after PathGuard authorizes the path.
// For a missing leaf, the snapshot still binds the existing parent and name.
func (f *LocalFileSystem) inspectEffect(path string, mutation bool) (effectPath, error) {
	operand := effectPath{path: path}
	info, err := effectInfo(path)
	if err != nil {
		return effectPath{}, err
	}
	operand.info = info
	if f.protection == nil {
		return operand, nil
	}
	operand.parent, err = effectInfo(filepath.Dir(path))
	if err != nil {
		return effectPath{}, security.ErrConfigurationAccess
	}
	operand.effective, err = effectivePathForComparison(path)
	if err != nil {
		return effectPath{}, security.ErrConfigurationAccess
	}
	if err := f.protection.Check(operand.effective, info, operand.parent, mutation); err != nil {
		return effectPath{}, err
	}
	if pathsEquivalent(operand.effective, path) {
		return operand, nil
	}
	return operand, f.protection.Check(path, info, operand.parent, mutation)
}

func effectInfo(path string) (os.FileInfo, error) {
	info, err := security.PathIdentity(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return info, err
}

func sameEffectInfo(left, right os.FileInfo) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return os.SameFile(left, right) && left.Mode().Type() == right.Mode().Type()
}

// recheckEffect uses the same policy again and rejects observable changes to
// the authorized resolution, parent or object before the native effect.
func (f *LocalFileSystem) recheckEffect(expected effectPath, mutation bool) error {
	if f.protection == nil {
		return nil
	}
	if err := f.protection.ValidateRoot(f.guard); err != nil {
		return err
	}
	relative, err := filepath.Rel(f.guard.Root(), expected.path)
	if err != nil {
		return security.ErrConfigurationAccess
	}
	if _, err := f.guard.ResolveForCreate(relative); err != nil {
		return security.ErrConfigurationAccess
	}
	current, err := f.inspectEffect(expected.path, mutation)
	if err != nil || !pathsEquivalent(expected.effective, current.effective) ||
		!sameEffectInfo(expected.info, current.info) || !sameEffectInfo(expected.parent, current.parent) {
		return security.ErrConfigurationAccess
	}
	return nil
}

func (f *LocalFileSystem) checkOpened(file *os.File, expected effectPath, mutation bool) error {
	if f.protection == nil {
		return nil
	}
	info, err := file.Stat()
	if err != nil || !sameEffectInfo(expected.info, info) {
		return security.ErrConfigurationAccess
	}
	return f.recheckEffect(expected, mutation)
}

// checkDestination also covers newly created destinations before content is written.
func (f *LocalFileSystem) checkDestination(file *os.File, target effectPath) error {
	if f.protection == nil {
		return nil
	}
	if target.info == nil {
		var err error
		target.info, err = file.Stat()
		if err != nil {
			return security.ErrConfigurationAccess
		}
	}
	return f.checkOpened(file, target, true)
}

func (f *LocalFileSystem) openDestination(target effectPath, mode os.FileMode, beforeEffect func() error) (*os.File, error) {
	if f.protection != nil && target.parent == nil {
		return nil, ErrNotFound
	}
	if err := f.recheckEffect(target, true); err != nil {
		return nil, err
	}
	if target.info == nil {
		if beforeEffect != nil {
			if err := beforeEffect(); err != nil {
				return nil, err
			}
		}
		file, err := os.OpenFile(target.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if errors.Is(err, os.ErrExist) {
			return nil, ErrFileExists
		}
		return file, err
	}

	file, err := os.OpenFile(target.path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	err = f.checkOpened(file, target, true)
	if err == nil && beforeEffect != nil {
		err = beforeEffect()
	}
	if err == nil {
		err = file.Truncate(0)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
