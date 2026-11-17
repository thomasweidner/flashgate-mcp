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
	maxBytes = min(maxBytes, f.limits.MaxFileSize, f.limits.MaxScanSize)

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

	// The scan ceiling is an I/O bound too: never read a max+1 probe beyond
	// it. A fresh handle stat detects growth without consuming content bytes;
	// at that ceiling its extent must also establish that no prefix was returned.
	content, err := readAtMost(file, maxBytes, f.limits.MaxScanSize)
	if err != nil {
		return nil, err
	}
	if err := f.checkOpened(file, operand, false); err != nil {
		return nil, err
	}
	current, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if current.Size() > maxBytes ||
		(int64(len(content)) == f.limits.MaxScanSize && current.Size() != int64(len(content))) {
		return nil, ErrFileTooLarge
	}
	return content, nil
}

func readAtMost(reader io.Reader, maxBytes, maxScanBytes int64) ([]byte, error) {
	if maxBytes <= 0 || maxScanBytes <= 0 {
		return nil, ErrFileTooLarge
	}
	probeLimit := maxBytes
	if probeLimit < math.MaxInt64 {
		probeLimit++
	}
	probeLimit = min(probeLimit, maxScanBytes)
	content, err := io.ReadAll(io.LimitReader(reader, probeLimit))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, ErrFileTooLarge
	}
	return content, nil
}
