package fs

import (
	"errors"
	"os"
)

// Stat returns filesystem metadata.
func (f *LocalFileSystem) Stat(path string, admission ...func() error) (Metadata, error) {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(path); err != nil {
		return Metadata{}, err
	}

	safePath, err := f.resolveExisting(path)
	if errors.Is(err, os.ErrNotExist) {
		safePath, err = f.guard.ResolveForCreate(path)
	}
	if err != nil {
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
	if info != nil {
		if err := f.checkFileType(operand, false); err != nil {
			return Metadata{}, err
		}
	}
	if err := f.recheckPath(operand); err != nil {
		return Metadata{}, err
	}
	if len(admission) != 0 {
		if err := admitEffect(admission); err != nil {
			return Metadata{}, err
		}
		if err := f.recheckPath(operand); err != nil {
			return Metadata{}, err
		}
	}

	if info == nil {
		return Metadata{}, ErrNotFound
	}
	return Metadata{
		Name:  safePath.Base(),
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}, nil
}
