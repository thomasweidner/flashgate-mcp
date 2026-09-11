package fs

import (
	"errors"
	"os"
	"time"
)

// Stat returns filesystem metadata.
func (f *LocalFileSystem) Stat(path string) (Metadata, error) {
	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metadata{}, ErrNotFound
		}
		return Metadata{}, err
	}

	info, err := os.Stat(safePath.String())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metadata{}, ErrNotFound
		}
		return Metadata{}, err
	}

	return Metadata{
		Name:         info.Name(),
		Type:         portablePathType(info.Mode()),
		IsDir:        info.IsDir(),
		Size:         info.Size(),
		ModifiedTime: info.ModTime().UTC().Format(time.RFC3339Nano),
		Permissions:  portablePermissions(info.Mode()),
	}, nil
}

func portablePathType(mode os.FileMode) string {
	switch {
	case mode.IsRegular():
		return "file"
	case mode.IsDir():
		return "directory"
	default:
		return "other"
	}
}
