package fs

import (
	"io"
	"math"
	"os"
)

// Read reads a file up to maxBytes bytes.
func (f *LocalFileSystem) Read(path string, maxBytes int64, admission ...func() error) ([]byte, error) {
	content, _, err := f.ReadMeasured(path, maxBytes, admission...)
	return content, err
}

// ReadMeasured preserves observed native bytes even when the result is rejected.
func (f *LocalFileSystem) ReadMeasured(path string, maxBytes int64, admission ...func() error) ([]byte, *uint64, error) {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(path); err != nil {
		return nil, nil, err
	}

	safePath, err := f.resolveExisting(path)
	if err != nil {
		return nil, nil, err
	}

	operand, err := f.inspectEffect(safePath.String(), false)
	if err != nil {
		return nil, nil, err
	}
	info := operand.info
	if err := f.checkOperandName(path, operand, false); err != nil {
		return nil, nil, err
	}
	if info == nil {
		return nil, nil, ErrNotFound
	}

	if info.IsDir() {
		return nil, nil, ErrPathIsDirectory
	}

	if maxBytes <= 0 {
		return nil, nil, ErrFileTooLarge
	}
	maxBytes = min(maxBytes, f.limits.MaxFileSize, f.limits.MaxScanSize)

	if info.Size() > maxBytes {
		return nil, nil, ErrFileTooLarge
	}

	if err := f.recheckEffect(operand, false); err != nil {
		return nil, nil, err
	}
	file, err := f.openNative(operand, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = file.Close() }()
	if err := f.checkOpened(file, operand, false); err != nil {
		return nil, nil, err
	}
	if len(admission) != 0 {
		if err := admitEffect(admission); err != nil {
			return nil, nil, err
		}
		if err := f.checkOpened(file, operand, false); err != nil {
			return nil, nil, err
		}
	}

	// The scan ceiling is an I/O bound too: never read a max+1 probe beyond
	// it. A fresh handle stat detects growth without consuming content bytes;
	// at that ceiling its extent must also establish that no prefix was returned.
	content, read, err := readAtMost(file, maxBytes, f.limits.MaxScanSize)
	if err != nil {
		return nil, read, err
	}
	if err := f.checkOpened(file, operand, false); err != nil {
		return nil, read, err
	}
	current, err := file.Stat()
	if err != nil {
		return nil, read, err
	}
	if current.Size() > maxBytes ||
		(int64(len(content)) == f.limits.MaxScanSize && current.Size() != int64(len(content))) {
		return nil, read, ErrFileTooLarge
	}
	return content, read, nil
}

func readAtMost(reader io.Reader, maxBytes, maxScanBytes int64) ([]byte, *uint64, error) {
	if maxBytes <= 0 || maxScanBytes <= 0 {
		return nil, nil, ErrFileTooLarge
	}
	probeLimit := maxBytes
	if probeLimit < math.MaxInt64 {
		probeLimit++
	}
	probeLimit = min(probeLimit, maxScanBytes)
	content, err := io.ReadAll(io.LimitReader(reader, probeLimit))
	read := uint64(len(content))
	if err != nil {
		return nil, &read, err
	}
	if int64(len(content)) > maxBytes {
		return nil, &read, ErrFileTooLarge
	}
	return content, &read, nil
}
