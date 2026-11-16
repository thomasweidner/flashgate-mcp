package fs

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const (
	listBatchSize     = 64
	maxListResultCost = 16 * 1024 * 1024
	listEntryCost     = 32
)

var errReadDirFile = errors.New("cannot read directory entries from a file")

// List lists directory entries.
func (f *LocalFileSystem) List(path string) ([]Entry, error) {
	f.effects.Lock()
	defer f.effects.Unlock()

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return nil, err
	}

	var operand effectPath
	if f.protection != nil {
		operand, err = f.inspectPath(safePath.String())
		if err != nil {
			return nil, err
		}
		if operand.info == nil {
			return nil, ErrNotFound
		}
		if !operand.info.IsDir() {
			// Keep the directory-read failure on the existing redacted error
			// path while avoiding a content handle on a regular file.
			return nil, &os.PathError{Op: "readdir", Path: operand.path, Err: errReadDirFile}
		}
		if err := f.recheckPath(operand); err != nil {
			return nil, err
		}
	}
	dir, err := os.Open(safePath.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	if f.protection != nil {
		info, err := dir.Stat()
		if err != nil || !sameEffectInfo(operand.info, info) {
			return nil, security.ErrConfigurationAccess
		}
	}

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

	if err := f.recheckPath(operand); err != nil {
		return nil, err
	}
	slices.SortFunc(result, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}
