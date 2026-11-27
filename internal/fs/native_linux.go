//go:build linux

package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type nativePath struct {
	root   *os.Root
	parent *os.File
	name   string
}

func nativeOpenAt(parent *os.File, name string, flags int, mode os.FileMode) (*os.File, error) {
	return security.OpenLocalAt(parent, name, flags, mode)
}

func nativeExclusionVolume(string) (string, error) { return "/", nil }

func nativeExclusionName(path string) (string, error) { return filepath.Base(path), nil }

const exclusionCaseNeedsTopologyCheck = true

func nativeExclusionDirectory(path string) (bool, error) {
	base, err := os.Open("/")
	if err != nil {
		return false, err
	}
	defer func() { _ = base.Close() }()
	relative, err := filepath.Rel("/", path)
	if err != nil || !filepath.IsLocal(relative) {
		return false, os.ErrPermission
	}
	// Prove the path in the root mount. In particular a separately
	// configured bind mount must not evade a rule on its original name.
	dir, err := nativeOpenAt(base, relative, os.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = dir.Close() }()
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(dir.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return false, os.ErrPermission
	}
	var flags uint64
	// FS_IOC_GETFLAGS on the supported 64-bit Linux ABIs. Deny an ext
	// directory with FS_CASEFOLD_FL or an unclassified flag result.
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, dir.Fd(), 0x80086601, uintptr(unsafe.Pointer(&flags)))
	runtime.KeepAlive(dir)
	if errno != 0 || flags&0x40000000 != 0 {
		return false, os.ErrPermission
	}
	return false, nil
}

func bindNativePath(root *os.Root, relative, _ string) (*nativePath, error) {
	base, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer base.Close()
	parent, err := nativeOpenAt(base, filepath.Dir(relative), 0x200000|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	return &nativePath{root, parent, filepath.Base(relative)}, nil
}

func (p *nativePath) close()                           { _ = p.parent.Close(); _ = p.root.Close() }
func (p *nativePath) parentInfo() (os.FileInfo, error) { return p.parent.Stat() }
func (p *nativePath) info() (os.FileInfo, error) {
	return nativeInfo(nativeOpenAt(p.parent, p.name, 0x200000, 0))
}
func (p *nativePath) matches(parent, entry os.FileInfo) error {
	currentParent, err := p.parent.Stat()
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
	return nativeOpenAt(p.parent, p.name, flags|syscall.O_NONBLOCK, mode)
}
func (p *nativePath) mkdir(mode os.FileMode) error {
	return syscall.Mkdirat(int(p.parent.Fd()), p.name, uint32(mode.Perm()))
}
func (p *nativePath) remove(directory bool) error {
	name, err := syscall.BytePtrFromString(p.name)
	if err != nil {
		return err
	}
	var flags uintptr
	if directory {
		flags = 0x200
	}
	_, _, errno := syscall.Syscall(syscall.SYS_UNLINKAT, p.parent.Fd(), uintptr(unsafe.Pointer(name)), flags)
	runtime.KeepAlive(p.parent)
	if errno != 0 {
		return errno
	}
	return nil
}
func (p *nativePath) rename(target *nativePath) error {
	return syscall.Renameat(int(p.parent.Fd()), p.name, int(target.parent.Fd()), target.name)
}
