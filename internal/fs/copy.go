package fs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type copyEntry struct {
	rel  string
	info fs.FileInfo
}

// Copy copies a regular file or a bounded directory tree.
func (f *LocalFileSystem) Copy(source string, target string, overwrite bool) error {
	sourcePath, err := f.guard.ResolveExisting(source)
	if err != nil {
		return err
	}
	targetPath, err := f.guard.ResolveForCreate(target)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(sourcePath.String())
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		if sourceInfo.Size() > f.limits.MaxCopyBytes {
			return ErrLimitExceeded
		}
		if err := ensureTargetPolicy(targetPath.String(), overwrite); err != nil {
			return err
		}
		return copyRegularFile(sourcePath.String(), targetPath.String(), sourceInfo.Mode(), overwrite)
	}

	return f.copyDirectory(source, target, sourcePath.String(), targetPath.String())
}

func (f *LocalFileSystem) copyDirectory(source, target, sourcePath, targetPath string) error {
	effectiveSource, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return err
	}
	effectiveTarget, err := effectivePathForComparison(targetPath)
	if err != nil {
		return err
	}
	if pathsEquivalent(effectiveSource, effectiveTarget) {
		return ErrSamePath
	}
	if isStrictDescendant(effectiveSource, effectiveTarget) {
		return ErrMoveIntoSelf
	}
	if _, err := os.Stat(targetPath); err == nil {
		return ErrFileExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	entries, err := f.preflightDirectoryCopy(source, sourcePath)
	if err != nil {
		return err
	}
	if err := os.Mkdir(targetPath, entries[0].info.Mode().Perm()); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}
		return err
	}
	createdTarget, err := os.Stat(targetPath)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		currentTarget, statErr := os.Stat(targetPath)
		if !complete && statErr == nil && os.SameFile(createdTarget, currentTarget) {
			_ = os.RemoveAll(targetPath)
		}
	}()

	for _, entry := range entries[1:] {
		sourceEntry := filepath.Join(sourcePath, entry.rel)
		targetEntry := filepath.Join(targetPath, entry.rel)
		if entry.info.IsDir() {
			err = os.Mkdir(targetEntry, entry.info.Mode().Perm())
		} else {
			err = copyRegularFile(sourceEntry, targetEntry, entry.info.Mode(), false)
		}
		if err != nil {
			return err
		}
	}
	complete = true
	return nil
}

func (f *LocalFileSystem) preflightDirectoryCopy(source, sourcePath string) ([]copyEntry, error) {
	entries := make([]copyEntry, 0)
	var totalBytes int64
	err := filepath.WalkDir(sourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if rel != "." {
			if len(entries) >= f.limits.MaxCopyEntries {
				return ErrLimitExceeded
			}
			if _, err := f.guard.ResolveExisting(filepath.Join(source, rel)); err != nil {
				return err
			}
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return ErrLimitExceeded
		}
		if info.Mode().IsRegular() {
			if info.Size() > f.limits.MaxCopyBytes-totalBytes {
				return ErrLimitExceeded
			}
			totalBytes += info.Size()
		}
		entries = append(entries, copyEntry{rel: rel, info: info})
		return nil
	})
	return entries, err
}

func copyRegularFile(sourcePath, targetPath string, mode fs.FileMode, overwrite bool) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	targetFile, err := os.OpenFile(targetPath, flags, mode.Perm())
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}
		return err
	}
	_, copyErr := io.Copy(targetFile, sourceFile)
	closeErr := targetFile.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func ensureTargetPolicy(targetPath string, overwrite bool) error {
	info, err := os.Stat(targetPath)
	if err == nil {
		if !overwrite {
			return ErrFileExists
		}
		if info.IsDir() {
			return ErrPathIsDirectory
		}
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
