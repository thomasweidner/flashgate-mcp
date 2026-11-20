package fs

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type effectPath struct {
	path        string
	info        os.FileInfo
	parent      os.FileInfo
	entryParent os.FileInfo
	effective   string
	links       []security.LinkIdentity
	structural  bool
	leafLink    bool
}

// inspectPath takes a metadata snapshot after PathGuard authorizes the path.
// For a missing leaf, the snapshot still binds the existing parent and name.
func (f *LocalFileSystem) inspectPath(path string) (effectPath, error) {
	operand := effectPath{path: path}
	var err error
	operand.effective, err = effectivePathForComparison(path)
	if err != nil {
		if errors.Is(err, security.ErrSymlinkDenied) || errors.Is(err, security.ErrReparsePointDenied) {
			return effectPath{}, err
		}
		return effectPath{}, security.ErrConfigurationAccess
	}
	operand.links, err = f.guard.LinkIdentities(path)
	if err != nil {
		return effectPath{}, err
	}
	operand.leafLink = security.HasLeafLink(path, operand.links)
	bound, err := f.bindNative(path, false)
	if errors.Is(err, os.ErrNotExist) {
		// Missing parent chains are inspected structurally by Mkdir before
		// creating each individual component through a fresh native binding.
		return operand, nil
	}
	if err != nil {
		return effectPath{}, err
	}
	defer bound.close()
	operand.info, err = bound.info()
	if err != nil {
		return effectPath{}, err
	}
	if operand.info != nil && operand.info.Mode()&os.ModeSymlink != 0 {
		// A dangling permitted link has no target object. Namespace operations
		// retain its separate LinkIdentity; content operations cannot open it.
		operand.info = nil
	}
	operand.parent, err = bound.parentInfo()
	if err != nil {
		return effectPath{}, err
	}
	operand.entryParent = operand.parent
	if operand.leafLink && !pathsEquivalent(path, f.guard.Root()) {
		// Content follows the allowed target, while unlink/rename affect the
		// link entry. Those parents may be different objects in the same root.
		entry, err := f.bindNative(path, true)
		if err != nil {
			return effectPath{}, err
		}
		defer entry.close()
		operand.entryParent, err = entry.parentInfo()
		if err != nil {
			return effectPath{}, err
		}
	}
	return operand, nil
}

func (f *LocalFileSystem) inspectEffect(path string, mutation bool) (effectPath, error) {
	return f.inspectEffectContext(path, mutation, false)
}

// Only implicit descendants of a selected tree effect use structural hidden
// handling. Client-selected operands always retain their original policy.
func (f *LocalFileSystem) inspectStructuralEffect(path string) (effectPath, error) {
	return f.inspectEffectContext(path, true, true)
}

func (f *LocalFileSystem) inspectEffectContext(path string, mutation, structural bool) (effectPath, error) {
	operand, err := f.inspectPath(path)
	if err != nil {
		return operand, err
	}
	operand.structural = structural
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
	return os.SameFile(left, right) && left.Mode().Type() == right.Mode().Type() && security.SamePolicyMetadata(left, right)
}

// recheckPath rejects observable changes to the authorized root, resolution,
// parent or object. Metadata operations may inspect the protected resource.
func (f *LocalFileSystem) recheckPath(expected effectPath) error {
	if f.protection != nil {
		if err := f.protection.ValidateRoot(); err != nil {
			return err
		}
	}
	var err error
	if expected.structural {
		_, err = f.guard.RevalidateEffect(expected.path)
	} else {
		var relative string
		relative, err = filepath.Rel(f.guard.Root(), expected.path)
		if err == nil {
			_, err = f.guard.ResolveForCreate(relative)
		}
	}
	if err != nil {
		return security.ErrConfigurationAccess
	}
	current, err := f.inspectPath(expected.path)
	if err != nil || !pathsEquivalent(expected.effective, current.effective) ||
		!sameEffectInfo(expected.info, current.info) || !sameEffectInfo(expected.parent, current.parent) ||
		!sameEffectInfo(expected.entryParent, current.entryParent) || expected.leafLink != current.leafLink ||
		!security.SameLinks(expected.links, current.links) {
		return security.ErrConfigurationAccess
	}
	return nil
}

func (f *LocalFileSystem) recheckEffect(expected effectPath, mutation bool) error {
	if err := f.recheckPath(expected); err != nil {
		return err
	}
	return f.checkEffect(expected, mutation)
}

func (f *LocalFileSystem) checkOpened(file *os.File, expected effectPath, mutation bool) error {
	info, err := file.Stat()
	if err != nil || !sameEffectInfo(expected.info, info) {
		return security.ErrConfigurationAccess
	}
	return f.recheckEffect(expected, mutation)
}

// checkDestination also covers newly created destinations before content is written.
func (f *LocalFileSystem) checkDestination(file *os.File, target effectPath) error {
	if target.info == nil {
		var err error
		target.info, err = file.Stat()
		if err != nil {
			return security.ErrConfigurationAccess
		}
	}
	return f.checkOpened(file, target, true)
}

func (f *LocalFileSystem) openDestination(target effectPath, mode os.FileMode, beforeEffect func() error, sourceChecks ...func() error) (*os.File, error) {
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
		if err := f.recheckEffect(target, true); err != nil {
			return nil, err
		}
		file, err := f.openNative(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode, sourceChecks...)
		if errors.Is(err, os.ErrExist) {
			return nil, ErrFileExists
		}
		return file, err
	}

	file, err := f.openNative(target, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	err = f.checkOpened(file, target, true)
	if err == nil && beforeEffect != nil {
		err = beforeEffect()
	}
	if err == nil {
		f.interceptNative("truncate")
		for _, check := range sourceChecks {
			if check != nil {
				if err = check(); err != nil {
					break
				}
			}
		}
	}
	if err == nil {
		err = f.checkOpened(file, target, true)
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
