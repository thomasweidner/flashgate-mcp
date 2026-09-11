package fs

import (
	"errors"
	"io"
	"os"
)

// EditRange replaces the half-open byte range [startByte,endByte) in an
// existing file. Both the source and resulting file are bounded by the
// configured write limit.
func (f *LocalFileSystem) EditRange(path string, startByte, endByte int64, content []byte) (int64, error) {
	if startByte < 0 || endByte < startByte || int64(len(content)) > f.limits.MaxWriteBytes {
		return 0, ErrLimitExceeded
	}

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(safePath.String())
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, ErrPathIsDirectory
	}
	if info.Size() > f.limits.MaxWriteBytes || startByte > info.Size() || endByte > info.Size() {
		return 0, ErrLimitExceeded
	}
	resultSize := info.Size() - (endByte - startByte) + int64(len(content))
	if resultSize > f.limits.MaxWriteBytes {
		return 0, ErrLimitExceeded
	}

	existing, err := os.ReadFile(safePath.String())
	if err != nil {
		return 0, err
	}
	result := make([]byte, 0, resultSize)
	result = append(result, existing[:startByte]...)
	result = append(result, content...)
	result = append(result, existing[endByte:]...)
	if err := os.WriteFile(safePath.String(), result, info.Mode().Perm()); err != nil {
		return 0, err
	}
	return resultSize, nil
}

// Write writes a file. Existing files are only overwritten when overwrite is true.
func (f *LocalFileSystem) Write(path string, content []byte, overwrite bool) error {
	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return err
	}

	if int64(len(content)) > f.limits.MaxWriteBytes {
		return ErrLimitExceeded
	}

	info, err := os.Stat(safePath.String())
	if err == nil {
		if info.IsDir() {
			return ErrPathIsDirectory
		}

		if !overwrite {
			return ErrFileExists
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(safePath.String(), flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}

		return err
	}
	defer file.Close()

	written, err := file.Write(content)
	if err != nil {
		return err
	}

	if written != len(content) {
		return io.ErrShortWrite
	}

	return nil
}

// Mkdir creates a directory and any missing parent directories and reports whether the leaf was created.
func (f *LocalFileSystem) Mkdir(path string) (bool, error) {
	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return false, err
	}
	if pathsEquivalent(safePath.String(), f.guard.Root()) {
		return false, nil
	}

	parent := safePath.Dir()
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return false, err
	}

	if err := os.Mkdir(safePath.String(), 0o700); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrExist) {
		return false, err
	}

	info, err := os.Stat(safePath.String())
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, ErrPathIsNotDirectory
	}

	return false, nil
}
