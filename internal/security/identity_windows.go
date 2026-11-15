//go:build windows

package security

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// PathIdentity returns handle-based metadata so os.SameFile does not need
// to look up a Windows path again to obtain its ID.
func PathIdentity(path string) (os.FileInfo, error) {
	name := path
	if filepath.IsAbs(path) && len(filepath.VolumeName(path)) == 2 {
		name = `\\?\` + path
	}
	pointer, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(pointer, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|
		syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	defer file.Close()
	return file.Stat()
}

func configPathEqual(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func configNameEqual(left, right string) bool {
	left, _, _ = strings.Cut(left, ":")
	return strings.EqualFold(strings.TrimRight(left, ". "), right)
}
