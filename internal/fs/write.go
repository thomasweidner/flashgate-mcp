package fs

import (
	"errors"
	"io"
	"os"
)

// Write writes a file. Existing files are only overwritten when overwrite is true.
func (f *LocalFileSystem) Write(path string, content []byte, overwrite bool) error {
	mode := WriteCreateOnly
	if overwrite {
		mode = WriteUpsert
	}
	return f.WriteWithMode(path, content, mode)
}

// WriteWithMode writes a file subject to an explicit existence precondition.
func (f *LocalFileSystem) WriteWithMode(path string, content []byte, mode WriteMode) error {
	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return err
	}

	if int64(len(content)) > f.limits.MaxWriteBytes {
		return ErrLimitExceeded
	}

	if mode != WriteCreateOnly && mode != WriteReplaceOnly && mode != WriteUpsert {
		return ErrUnsupportedWriteMode
	}

	info, err := os.Stat(safePath.String())
	if err == nil {
		if info.IsDir() {
			return ErrPathIsDirectory
		}

		if mode == WriteCreateOnly {
			return ErrFileExists
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if mode == WriteReplaceOnly {
			return ErrNotFound
		}
	} else {
		return err
	}

	flags := os.O_WRONLY
	switch mode {
	case WriteCreateOnly:
		flags |= os.O_CREATE | os.O_EXCL
	case WriteReplaceOnly:
		flags |= os.O_TRUNC
	case WriteUpsert:
		flags |= os.O_CREATE | os.O_TRUNC
	}
	if mode != WriteCreateOnly {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(safePath.String(), flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
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
