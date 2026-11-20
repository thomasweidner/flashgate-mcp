package fs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/resource"
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

	operand, err := f.inspectPath(safePath.String())
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
	dir, err := f.openNative(operand, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	info, err := dir.Stat()
	if err != nil || !sameEffectInfo(operand.info, info) {
		return nil, security.ErrConfigurationAccess
	}

	result := make([]Entry, 0)
	meter, err := resource.NewMeter(f.limits.ResourceLimits())
	if err != nil {
		return nil, err
	}
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
			if err := meter.Charge(resource.Scan, int64(len(name))); err != nil {
				return nil, err
			}
			// Only a known regular entry can be filtered before resolution.
			// A permitted directory link keeps its structural semantics.
			if dirEntry.Type().IsRegular() && (!f.fileTypes.AllowsName(name) || ambiguousNativeFileName(name)) {
				continue
			}
			allowed, err := f.guard.AllowListEntry(safePath, name)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
			entryPath, err := f.guard.ResolveExisting(filepath.Join(path, name))
			if err != nil {
				return nil, err
			}
			entryOperand, err := f.inspectPath(entryPath.String())
			if err != nil {
				return nil, err
			}
			if entryOperand.info == nil {
				return nil, ErrFileTypeDenied
			}
			if err := f.checkFileType(entryOperand, false); err != nil {
				if errors.Is(err, ErrFileTypeDenied) {
					continue
				}
				return nil, err
			}
			if err := f.recheckPath(entryOperand); err != nil {
				return nil, err
			}
			// Bound retained raw names and per-entry storage even when the
			// configured entry limit is raised. The adapter also checks wire size.
			cost := len(name) + listEntryCost
			if cost > resultRemaining {
				return nil, ErrLimitExceeded
			}
			// Use the identity-bound metadata already checked above. A deferred
			// DirEntry.Info call could reopen a directory name after an exchange.
			result = append(result, Entry{Name: name, IsDir: entryOperand.info.IsDir(), Size: entryOperand.info.Size()})
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
