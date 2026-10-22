//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func transientStageRename(err error) bool {
	var code syscall.Errno
	return errors.As(err, &code) && (code == 5 || code == 32)
}

func stageRenameDiagnostic(err error) error {
	var code syscall.Errno
	if errors.As(err, &code) {
		return fmt.Errorf("Win32=%d: %w", code, err)
	}
	return err
}

func platformStageRenameOps() stageRenameOps {
	return stageRenameOps{os.Rename, time.Now, time.Sleep, transientStageRename, 8, 2 * time.Second}
}

// Lstat's Windows file ID is lazy and may later reopen a replaced path.
// File.Stat freezes it from this no-follow handle, closed before returning.
func stageDirectory(path string) (os.FileInfo, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(name, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	info, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		return nil, err
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || !info.IsDir() || data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, fmt.Errorf("not a real non-reparse directory: %q", path)
	}
	return info, nil
}
