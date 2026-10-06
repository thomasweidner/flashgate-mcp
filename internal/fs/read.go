package fs

import (
	"io"
	"math"
	"os"
)

// Read reads a file up to maxBytes bytes.
func (f *LocalFileSystem) Read(path string, maxBytes int64) ([]byte, error) {
	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(safePath.String())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, ErrPathIsDirectory
	}

	if maxBytes <= 0 {
		return nil, ErrFileTooLarge
	}

	if info.Size() > maxBytes {
		return nil, ErrFileTooLarge
	}

	// Read from the validated open handle and retain one detection byte so a file
	// that grows after Stat cannot make this operation allocate or return more
	// than the caller's limit.
	readLimit := maxBytes
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	content, err := io.ReadAll(io.LimitReader(file, readLimit))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, ErrFileTooLarge
	}

	return content, nil
}
