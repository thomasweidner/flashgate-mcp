package fs

import (
	"errors"
	"os"
)

// Stat returns filesystem metadata.
func (f *LocalFileSystem) Stat(path string) (Metadata, error) {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(path); err != nil {
		return Metadata{}, err
	}

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metadata{}, ErrNotFound
		}
		return Metadata{}, err
	}

	operand, err := f.inspectPath(safePath.String())
	if err != nil {
		return Metadata{}, err
	}
	info := operand.info
	if err := f.checkOperandName(path, operand, false); err != nil {
		return Metadata{}, err
	}
	if info == nil {
		return Metadata{}, ErrNotFound
	}
	if err := f.checkFileType(operand, false); err != nil {
		return Metadata{}, err
	}
	if err := f.recheckPath(operand); err != nil {
		return Metadata{}, err
	}

	return Metadata{
		Name:  safePath.Base(),
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}, nil
}
