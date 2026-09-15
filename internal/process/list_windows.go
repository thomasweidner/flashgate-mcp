//go:build windows

package process

import (
	"context"
	"errors"
	"syscall"
	"unsafe"
)

const (
	th32csSnapProcess = 0x00000002
	maxPathChars      = 260
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelpSnapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW        = kernel32.NewProc("Process32FirstW")
	procProcess32NextW         = kernel32.NewProc("Process32NextW")
)

type processEntry32 struct {
	Size              uint32
	Usage             uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	Threads           uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [maxPathChars]uint16
}

func listProcesses(ctx context.Context) ([]Entry, error) {
	handle, _, callErr := procCreateToolhelpSnapshot.Call(th32csSnapProcess, 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return nil, callErr
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	current := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	ok, _, callErr := procProcess32FirstW.Call(handle, uintptr(unsafe.Pointer(&current)))
	if ok == 0 {
		return nil, callErr
	}

	var entries []Entry
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := syscall.UTF16ToString(current.ExeFile[:])
		if current.ProcessID != 0 && name != "" {
			entries = append(entries, Entry{PID: current.ProcessID, Name: name})
		}
		current.Size = uint32(unsafe.Sizeof(processEntry32{}))
		ok, _, callErr = procProcess32NextW.Call(handle, uintptr(unsafe.Pointer(&current)))
		if ok == 0 {
			if errors.Is(callErr, syscall.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, callErr
		}
	}
	return entries, nil
}
