//go:build linux

package security

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// OpenLocalAt opens below a trusted directory handle without crossing mounts
// or resolving links. Both supported Linux architectures require openat2.
func OpenLocalAt(parent *os.File, name string, flags int, mode os.FileMode) (*os.File, error) {
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
	// 437 is openat2 on both released 64-bit ABIs.
	fd, _, errno := syscall.Syscall6(437, parent.Fd(), uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&how)), unsafe.Sizeof(how), 0, 0)
	runtime.KeepAlive(parent)
	if errno != 0 {
		if errors.Is(errno, syscall.ELOOP) || errors.Is(errno, syscall.EXDEV) || errors.Is(errno, syscall.ENOSYS) {
			return nil, os.ErrPermission
		}
		return nil, &os.PathError{Op: "openat2", Path: name, Err: errno}
	}
	return os.NewFile(fd, filepath.Join(parent.Name(), name)), nil
}
