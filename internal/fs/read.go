package fs

import (
	"io"
	"math"
	"os"
)

// Read reads a file up to maxBytes bytes.
func (f *LocalFileSystem) Read(path string, maxBytes int64) ([]byte, error) {
	f.effects.Lock()
	defer f.effects.Unlock()

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return nil, err
	}

	operand, err := f.inspectEffect(safePath.String(), false)
	if err != nil {
		return nil, err
	}
	info := operand.info
	if info == nil {
		return nil, ErrNotFound
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

	if err := f.recheckEffect(operand, false); err != nil {
		return nil, err
	}
	file, err := os.Open(operand.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	if err := f.checkOpened(file, operand, false); err != nil {
		return nil, err
	}

	content, err := readAtMost(file, maxBytes)
	if err != nil {
		return nil, err
	}
	if err := f.checkOpened(file, operand, false); err != nil {
		return nil, err
	}
	return content, nil
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
