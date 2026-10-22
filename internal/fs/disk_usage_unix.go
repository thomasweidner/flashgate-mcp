//go:build !windows

package fs

import "syscall"

func diskUsageForPath(path string) (DiskUsage, error) {
	var statistics syscall.Statfs_t
	if err := syscall.Statfs(path, &statistics); err != nil {
		return DiskUsage{}, err
	}

	blockSize := uint64(statistics.Bsize)
	total := statistics.Blocks * blockSize
	free := statistics.Bfree * blockSize
	available := statistics.Bavail * blockSize
	return DiskUsage{
		TotalBytes:     total,
		UsedBytes:      total - free,
		AvailableBytes: available,
	}, nil
}
