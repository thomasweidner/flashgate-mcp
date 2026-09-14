package fs

import "os"

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

// ReadRange reads at most length bytes starting at the zero-based byte offset.
// A negative offset addresses bytes relative to the end of the file, so -N
// reads the final N-byte window. Requests beyond EOF return an empty slice.
func (f *LocalFileSystem) ReadRange(path string, offset, length, maxBytes int64) ([]byte, error) {
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
	if length < 0 || maxBytes <= 0 || length > maxBytes {
		return nil, ErrFileTooLarge
	}
	if offset < 0 {
		offset = info.Size() + offset
		if offset < 0 {
			offset = 0
		}
	}
	if offset >= info.Size() || length == 0 {
		return []byte{}, nil
	}
	if remaining := info.Size() - offset; length > remaining {
		length = remaining
	}

	content := make([]byte, length)
	read, err := file.ReadAt(content, offset)
	if err != nil && read != len(content) {
		return nil, err
	}
	return content[:read], nil
}
