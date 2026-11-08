package fs

import (
	"io"
	"os"
	"slices"
	"strings"
)

const (
	listBatchSize     = 64
	maxListResultCost = 16 * 1024 * 1024
	listEntryCost     = 32
)

// List lists directory entries.
func (f *LocalFileSystem) List(path string) ([]Entry, error) {
	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return nil, err
	}

	dir, err := os.Open(safePath.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()

	result := make([]Entry, 0)
	scanRemaining := f.limits.MaxListEntries
	resultRemaining := maxListResultCost
	for {
		// Once the budget is exhausted, read one entry only to distinguish
		// an exact fit from overflow. Never inspect that entry's policy.
		batch, readErr := dir.ReadDir(min(listBatchSize, max(1, scanRemaining)))
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}
		if len(batch) > scanRemaining {
			return nil, ErrLimitExceeded
		}
		scanRemaining -= len(batch)
		for _, dirEntry := range batch {
			name := dirEntry.Name()
			allowed, err := f.guard.AllowListEntry(safePath, name)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
			// Bound retained raw names and per-entry storage even when the
			// configured entry limit is raised. The adapter also checks wire size.
			cost := len(name) + listEntryCost
			if cost > resultRemaining {
				return nil, ErrLimitExceeded
			}
			info, err := dirEntry.Info()
			if err != nil {
				return nil, err
			}
			result = append(result, Entry{Name: name, IsDir: dirEntry.IsDir(), Size: info.Size()})
			resultRemaining -= cost
		}
		if readErr == io.EOF {
			break
		}
	}

	slices.SortFunc(result, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}
