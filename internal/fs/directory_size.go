package fs

import (
	"context"
	iofs "io/fs"
	"os"
	"path/filepath"
)

const (
	defaultMaxDirectorySizeEntries = 10000
	defaultMaxDirectorySizeBytes   = int64(10 * 1024 * 1024 * 1024)
)

// DirectorySizeResult is the bounded aggregate produced by DirectorySize.
type DirectorySizeResult struct {
	Bytes       int64 `json:"bytes"`
	Files       int   `json:"files"`
	Directories int   `json:"directories"`
	Entries     int   `json:"entries"`
}

// DirectorySizeProgress is a point-in-time snapshot suitable for a job manager.
type DirectorySizeProgress = DirectorySizeResult

// DirectorySize scans a directory without following directory symlinks. The
// callback is optional and receives monotonic progress after every counted entry.
func (f *LocalFileSystem) DirectorySize(ctx context.Context, path string, progress func(DirectorySizeProgress)) (DirectorySizeResult, error) {
	return f.directorySize(ctx, path, progress, defaultMaxDirectorySizeEntries, defaultMaxDirectorySizeBytes)
}

func (f *LocalFileSystem) directorySize(ctx context.Context, path string, progress func(DirectorySizeProgress), maxEntries int, maxBytes int64) (DirectorySizeResult, error) {
	var result DirectorySizeResult
	safeRoot, err := f.guard.ResolveExisting(path)
	if err != nil {
		return result, err
	}

	info, err := os.Stat(safeRoot.String())
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, ErrPathIsNotDirectory
	}

	err = filepath.WalkDir(safeRoot.String(), func(current string, entry iofs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == safeRoot.String() {
			return nil
		}

		parent, err := filepath.Rel(safeRoot.String(), filepath.Dir(current))
		if err != nil {
			return err
		}
		parentPath, err := f.guard.ResolveExisting(filepath.Join(path, parent))
		if err != nil {
			return err
		}
		allowed, err := f.guard.AllowListEntry(parentPath, entry.Name())
		if err != nil {
			return err
		}
		if !allowed {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		result.Entries++
		if result.Entries > maxEntries {
			return ErrLimitExceeded
		}
		if entry.IsDir() {
			result.Directories++
		} else {
			entryInfo, err := entry.Info()
			if err != nil {
				return err
			}
			result.Files++
			result.Bytes += entryInfo.Size()
			if result.Bytes > maxBytes {
				return ErrLimitExceeded
			}
		}
		if progress != nil {
			progress(result)
		}
		return nil
	})
	return result, err
}
