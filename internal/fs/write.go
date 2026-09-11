package fs

import (
	"errors"
	"io"
	"os"
)

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

// Append appends content to a file, creating it when it does not exist.
func (f *LocalFileSystem) Append(path string, content []byte) error {
	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return err
	}

	if int64(len(content)) > f.limits.MaxWriteBytes {
		return ErrLimitExceeded
	}
	if info, statErr := os.Stat(safePath.String()); statErr == nil && info.IsDir() {
		return ErrPathIsDirectory
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	file, err := os.OpenFile(safePath.String(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return ErrPathIsDirectory
	}

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
