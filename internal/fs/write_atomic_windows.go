package fs

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var (
	kernel32MoveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	errAlreadyExists   = syscall.Errno(183)
)

func publishAtomicWrite(stage string, target string, overwrite bool) error {
	stagePtr, err := syscall.UTF16PtrFromString(stage)
	if err != nil {
		return err
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	flags := uintptr(moveFileWriteThrough)
	if overwrite {
		flags |= moveFileReplaceExisting
	}
	result, _, callErr := kernel32MoveFileEx.Call(
		uintptr(unsafe.Pointer(stagePtr)),
		uintptr(unsafe.Pointer(targetPtr)),
		flags,
	)
	if result != 0 {
		return nil
	}
	if errors.Is(callErr, errAlreadyExists) || errors.Is(callErr, syscall.ERROR_FILE_EXISTS) {
		return os.ErrExist
	}
	return callErr
}
