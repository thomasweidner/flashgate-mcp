//go:build linux

package audit

import (
	"errors"
	"os"
	"syscall"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func lockFile(file *os.File, exclusive bool) (bool, error) {
	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX
	}
	err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlockFile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func nativeSamePath(a, b string) bool { return a == b }

func makeStoreDirectory(parent *os.Root, name string) error {
	return parent.Mkdir(name, 0o700)
}

func makeStorePartition(root *os.Root, name string) error {
	return makeStoreDirectory(root, name)
}

func createStoreFile(root *os.Root, name string) (*os.File, error) {
	return secureOpen(root, name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600, false)
}

func secureOpen(root *os.Root, name string, flags int, mode os.FileMode, directory bool) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	flags |= syscall.O_NONBLOCK
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	file, err := security.OpenLocalAt(parent, name, flags, mode)
	if err != nil {
		return nil, err
	}
	if !ownedNode(file, directory) {
		file.Close()
		return nil, ErrUnavailable
	}
	return file, nil
}

func ownedNode(file *os.File, directory bool) bool {
	info, err := file.Stat()
	if err != nil || info.IsDir() != directory || !directory && !info.Mode().IsRegular() {
		return false
	}
	node, ok := info.Sys().(*syscall.Stat_t)
	return ok && node.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0o077 == 0 &&
		(directory || node.Nlink == 1)
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := secureOpen(root, name, os.O_RDONLY, 0, true)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
