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

// inspectPath takes a metadata snapshot after PathGuard authorizes the path.
// For a missing leaf, the snapshot still binds the existing parent and name.
func (f *LocalFileSystem) inspectPath(path string) (effectPath, error) {
	operand := effectPath{path: path}
	info, err := effectInfo(path)
	if err != nil {
		return effectPath{}, err
	}
	operand.info = info
	if !f.checksIdentity() {
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
	return operand, nil
}

func (f *LocalFileSystem) inspectEffect(path string, mutation bool) (effectPath, error) {
	operand, err := f.inspectPath(path)
	if err != nil {
		return operand, err
	}
	return operand, f.checkEffect(operand, mutation)
}

func (f *LocalFileSystem) checkEffect(operand effectPath, mutation bool) error {
	if f.protection == nil {
		return f.checkFileType(operand, false)
	}
	if err := f.protection.Check(operand.effective, operand.info, operand.parent, mutation); err != nil {
		return err
	}
	if pathsEquivalent(operand.effective, operand.path) {
		return f.checkFileType(operand, false)
	}
	if err := f.protection.Check(operand.path, operand.info, operand.parent, mutation); err != nil {
		return err
	}
	return f.checkFileType(operand, false)
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

// recheckPath rejects observable changes to the authorized root, resolution,
// parent or object. Metadata operations may inspect the protected resource.
func (f *LocalFileSystem) recheckPath(expected effectPath) error {
	if !f.checksIdentity() {
		return nil
	}
	if f.protection != nil {
		if err := f.protection.ValidateRoot(); err != nil {
			return err
		}
	}
	relative, err := filepath.Rel(f.guard.Root(), expected.path)
	if err != nil {
		return security.ErrConfigurationAccess
	}
	if _, err := f.guard.ResolveForCreate(relative); err != nil {
		return security.ErrConfigurationAccess
	}
	current, err := f.inspectPath(expected.path)
	if err != nil || !pathsEquivalent(expected.effective, current.effective) ||
		!sameEffectInfo(expected.info, current.info) || !sameEffectInfo(expected.parent, current.parent) {
		return security.ErrConfigurationAccess
	}
	return nil
}

func (f *LocalFileSystem) recheckEffect(expected effectPath, mutation bool) error {
	if !f.checksIdentity() {
		return nil
	}
	if err := f.recheckPath(expected); err != nil {
		return err
	}
	return f.checkEffect(expected, mutation)
}

func (f *LocalFileSystem) checkOpened(file *os.File, expected effectPath, mutation bool) error {
	if !f.checksIdentity() {
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
	if !f.checksIdentity() {
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
	if err := f.checkFileType(target, true); err != nil {
		return nil, err
	}
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
