//go:build windows

package fs

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type nativePath struct {
	root, parent   *os.Root
	name, relative string
}

const exclusionCaseNeedsTopologyCheck = false

func nativeExclusionVolume(path string) (string, error) {
	return security.WindowsExclusionVolume(path)
}

func nativeExclusionName(path string) (string, error) {
	return security.WindowsExclusionName(path)
}

func nativeExclusionDirectory(path string) (bool, error) {
	return security.WindowsExclusionCase(path)
}

// The effect uses the opened parent, never an absolute pathname reopened
// after validation. Root's native traversal rejects absolute reparse targets;
// PathGuard additionally classifies every supported link and rejects junctions.
func bindNativePath(root *os.Root, relative, _ string) (*nativePath, error) {
	parent, err := root.OpenRoot(filepath.Dir(relative))
	if err != nil {
		return nil, err
	}
	return &nativePath{root: root, parent: parent, name: filepath.Base(relative), relative: relative}, nil
}

func (p *nativePath) close() { _ = p.parent.Close(); _ = p.root.Close() }
func (p *nativePath) parentInfo() (os.FileInfo, error) {
	return nativeInfo(p.parent.Open("."))
}
func (p *nativePath) info() (os.FileInfo, error) {
	info, err := p.parent.Lstat(p.name)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}
func (p *nativePath) matches(parent, entry os.FileInfo) error {
	currentParent, err := p.parentInfo()
	if err != nil || !sameEffectInfo(parent, currentParent) {
		return security.ErrConfigurationAccess
	}
	current, err := p.info()
	if err != nil || !sameEffectInfo(entry, current) {
		return security.ErrConfigurationAccess
	}
	return nil
}
func (p *nativePath) open(flags int, mode os.FileMode) (*os.File, error) {
	if flags&os.O_TRUNC != 0 || flags&os.O_CREATE != 0 && flags&os.O_EXCL == 0 {
		return nil, os.ErrPermission
	}
	// This documented Windows OpenFile flag opens the final reparse object
	// itself. The handle check rejects it before content I/O or truncation;
	// a racing leaf link is never followed, even under an allowing policy.
	file, err := p.parent.OpenFile(p.name, flags|syscall.FILE_FLAG_OPEN_REPARSE_POINT, mode)
	if err != nil {
		return nil, err
	}
	h := syscall.Handle(file.Fd())
	var info syscall.ByHandleFileInformation
	kind, kindErr := syscall.GetFileType(h)
	err = syscall.GetFileInformationByHandle(h, &info)
	if err != nil || kindErr != nil || kind != syscall.FILE_TYPE_DISK || info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = file.Close()
		return nil, os.ErrPermission
	}
	return file, nil
}
func (p *nativePath) mkdir(mode os.FileMode) error { return p.parent.Mkdir(p.name, mode) }
func (p *nativePath) remove(bool) error            { return p.parent.Remove(p.name) }
func (p *nativePath) rename(target *nativePath) error {
	// Root.Rename confines both operands. The API does not offer an atomic
	// identity-conditional rename; retain the documented independent-process
	// final namespace residual after the identity rechecks.
	return p.root.Rename(p.relative, target.relative)
}
