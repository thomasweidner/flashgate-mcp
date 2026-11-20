//go:build linux

package fs

import (
	"errors"
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

// Linux openat2 is required: NO_XDEV rejects bind mounts even when st_dev
// matches, and NO_SYMLINKS denies replacement links at the native boundary.
// There is deliberately no weaker openat/path fallback on older kernels.
func nativeOpenAt(parent *os.File, name string, flags int, mode os.FileMode) (*os.File, error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return nil, os.ErrPermission
	}
	p, err := syscall.BytePtrFromString(name)
	if err != nil {
		return nil, err
	}
	how := struct{ Flags, Mode, Resolve uint64 }{Flags: uint64(flags | syscall.O_CLOEXEC | syscall.O_NOFOLLOW), Resolve: 0xf}
	if flags&os.O_CREATE != 0 {
		how.Mode = uint64(mode.Perm())
	}
	// 437 is openat2 on the supported Linux amd64 and arm64 ABIs.
	fd, _, errno := syscall.Syscall6(437, parent.Fd(), uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&how)), unsafe.Sizeof(how), 0, 0)
	runtime.KeepAlive(parent)
	if errno != 0 {
		if errors.Is(errno, syscall.ELOOP) || errors.Is(errno, syscall.EXDEV) || errors.Is(errno, syscall.ENOSYS) {
			return nil, os.ErrPermission
		}
		return nil, &os.PathError{Op: "openat2", Path: name, Err: errno}
	}
	// os.File's directory entries use its name for deferred metadata. Keep
	// that name absolute even though the kernel operation is descriptor-relative.
	return os.NewFile(fd, filepath.Join(parent.Name(), name)), nil
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
