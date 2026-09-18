package fs

import (
	"errors"
	"os"
)

// DiskUsage returns privacy-safe capacity information for the filesystem that
// stores an existing path below the configured root.
func (f *LocalFileSystem) DiskUsage(path string) (DiskUsage, error) {
	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DiskUsage{}, ErrNotFound
		}
		return DiskUsage{}, err
	}

	usage, err := diskUsageForPath(safePath.String())
	if err != nil {
		return DiskUsage{}, err
	}
	if usage.AvailableBytes > usage.TotalBytes || usage.UsedBytes > usage.TotalBytes {
		return DiskUsage{}, ErrLimitExceeded
	}
	return usage, nil
}
