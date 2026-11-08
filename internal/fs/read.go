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

	info, err := os.Stat(safePath.String())
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

	file, err := os.Open(safePath.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return readAtMost(file, maxBytes)
}

func readAtMost(reader io.Reader, maxBytes int64) ([]byte, error) {
	probeLimit := maxBytes
	if probeLimit < math.MaxInt64 {
		probeLimit++
	}
	content, err := io.ReadAll(io.LimitReader(reader, probeLimit))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, ErrFileTooLarge
	}
	return content, nil
}
