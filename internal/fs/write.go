package fs

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Write writes a file. Existing files are only overwritten when overwrite is true.
func (f *LocalFileSystem) Write(path string, content []byte, overwrite bool) error {
	return f.WriteConditional(path, content, overwrite, WritePreconditions{})
}

// WriteConditional writes a file only when every supplied precondition matches.
func (f *LocalFileSystem) WriteConditional(path string, content []byte, overwrite bool, preconditions WritePreconditions) error {
	safePath, err := f.guard.ResolveForCreate(path)
	if err != nil {
		return err
	}

	if int64(len(content)) > f.limits.MaxWriteBytes {
		return ErrLimitExceeded
	}
	if !overwrite {
		info, statErr := os.Stat(safePath.String())
		if statErr == nil {
			if info.IsDir() {
				return ErrPathIsDirectory
			}
			return ErrFileExists
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
	}

	file, err := os.OpenFile(safePath.String(), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		if !matchesMissingTarget(preconditions) {
			return ErrWritePreconditionFailed
		}
		file, err = os.OpenFile(safePath.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	} else if err == nil {
		if err := checkExistingPreconditions(file, preconditions); err != nil {
			file.Close()
			return err
		}
		if err := file.Truncate(0); err != nil {
			file.Close()
			return err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			return err
		}
	} else if info, statErr := os.Stat(safePath.String()); statErr == nil && info.IsDir() {
		return ErrPathIsDirectory
	}

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

func matchesMissingTarget(preconditions WritePreconditions) bool {
	if preconditions.SHA256 != nil || preconditions.ModifiedTime != nil {
		return false
	}
	return preconditions.PathType == nil || *preconditions.PathType == "missing"
}

func checkExistingPreconditions(file *os.File, preconditions WritePreconditions) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return ErrPathIsDirectory
	}
	if !info.Mode().IsRegular() {
		return ErrWritePreconditionFailed
	}
	if preconditions.PathType != nil && *preconditions.PathType != "file" {
		return ErrWritePreconditionFailed
	}
	if preconditions.ModifiedTime != nil && !info.ModTime().Equal(*preconditions.ModifiedTime) {
		return ErrWritePreconditionFailed
	}
	if preconditions.SHA256 != nil {
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		actual := fmt.Sprintf("%x", hash.Sum(nil))
		if !strings.EqualFold(actual, *preconditions.SHA256) {
			return ErrWritePreconditionFailed
		}
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
