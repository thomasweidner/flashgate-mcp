package fs

import "os"

// List lists directory entries.
func (f *LocalFileSystem) List(path string) ([]Entry, error) {
	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(safePath.String())
	if err != nil {
		return nil, err
	}

	result := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		allowed, err := f.guard.AllowListEntry(safePath, dirEntry.Name())
		if err != nil {
			return nil, err
		}

		if !allowed {
			continue
		}

		if len(result) >= f.limits.MaxListEntries {
			return nil, ErrLimitExceeded
		}

		info, err := dirEntry.Info()
		if err != nil {
			return nil, err
		}

		result = append(result, Entry{
			Name:         dirEntry.Name(),
			IsDir:        dirEntry.IsDir(),
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
		})
	}

	return result, nil
}
