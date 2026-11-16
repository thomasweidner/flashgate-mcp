package fs

import (
	"errors"
	"os"
)

// Stat returns filesystem metadata.
func (f *LocalFileSystem) Stat(path string) (Metadata, error) {
	f.effects.Lock()
	defer f.effects.Unlock()

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
	if info == nil {
		return Metadata{}, ErrNotFound
	}
	if err := f.recheckPath(operand); err != nil {
		return Metadata{}, err
	}

	return Metadata{
		Name:  info.Name(),
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}, nil
}
