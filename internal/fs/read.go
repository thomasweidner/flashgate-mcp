package fs

import (
	"bytes"
	"io"
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

	return os.ReadFile(safePath.String())
}

// ReadLines reads an inclusive, one-based line range without retaining bytes
// before or after the selected window. Line terminators within the window are
// preserved exactly as stored in the file.
func (f *LocalFileSystem) ReadLines(path string, startLine, endLine, maxBytes, maxScanBytes int64) ([]byte, error) {
	if startLine < 1 || endLine < startLine || maxBytes <= 0 || maxScanBytes <= 0 {
		return nil, ErrLimitExceeded
	}

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

	var result bytes.Buffer
	buffer := make([]byte, 32*1024)
	line := int64(1)
	scanned := int64(0)
	for {
		read, readErr := file.Read(buffer)
		for _, value := range buffer[:read] {
			if scanned >= maxScanBytes {
				return nil, ErrFileTooLarge
			}
			scanned++
			if line >= startLine && line <= endLine {
				if int64(result.Len()) >= maxBytes {
					return nil, ErrFileTooLarge
				}
				result.WriteByte(value)
			}
			if value == '\n' {
				if line == endLine {
					return result.Bytes(), nil
				}
				line++
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return result.Bytes(), nil
			}
			return nil, readErr
		}
	}
}
