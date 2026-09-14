package fs

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
)

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
			Name:  dirEntry.Name(),
			IsDir: dirEntry.IsDir(),
			Size:  info.Size(),
		})
	}

	return result, nil
}

// ListPage returns one bounded page in platform-neutral ordinal name order.
// The fingerprint identifies all policy-visible entries and supports validated
// continuation without retaining directory contents in memory between calls.
func (f *LocalFileSystem) ListPage(path string, offset int, limit int) ([]Entry, string, int, error) {
	if offset < 0 || limit < 1 {
		return nil, "", 0, ErrLimitExceeded
	}
	if limit > f.limits.MaxListEntries {
		limit = f.limits.MaxListEntries
	}

	safePath, err := f.guard.ResolveExisting(path)
	if err != nil {
		return nil, "", 0, err
	}
	dirEntries, err := os.ReadDir(safePath.String())
	if err != nil {
		return nil, "", 0, err
	}

	hash := sha256.New()
	page := make([]Entry, 0, limit)
	visible := 0
	for _, dirEntry := range dirEntries {
		allowed, err := f.guard.AllowListEntry(safePath, dirEntry.Name())
		if err != nil {
			return nil, "", 0, err
		}
		if !allowed {
			continue
		}
		info, err := dirEntry.Info()
		if err != nil {
			return nil, "", 0, err
		}
		entry := Entry{Name: dirEntry.Name(), IsDir: dirEntry.IsDir(), Size: info.Size()}
		if err := binary.Write(hash, binary.BigEndian, uint64(len(entry.Name))); err != nil {
			return nil, "", 0, err
		}
		hash.Write([]byte(entry.Name))
		if entry.IsDir {
			hash.Write([]byte{1})
		} else {
			hash.Write([]byte{0})
		}
		if err := binary.Write(hash, binary.BigEndian, entry.Size); err != nil {
			return nil, "", 0, err
		}
		if visible >= offset && len(page) < limit {
			page = append(page, entry)
		}
		visible++
	}
	next := 0
	if offset+len(page) < visible {
		next = offset + len(page)
	}
	return page, hex.EncodeToString(hash.Sum(nil)), next, nil
}
