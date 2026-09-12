//go:build windows

package fs

import (
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func diskUsageForPath(path string) (DiskUsage, error) {
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return DiskUsage{}, err
	}

	var available, total, free uint64
	success, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPointer)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)),
	)
	if success == 0 {
		return DiskUsage{}, callErr
	}
	return DiskUsage{
		TotalBytes:     total,
		UsedBytes:      total - free,
		AvailableBytes: available,
	}, nil
}
