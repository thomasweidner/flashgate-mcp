package security

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const AuditTreeMaxEntries = 4096

var ErrAuditPath = fmt.Errorf("protected audit access denied: %w", os.ErrPermission)

// CheckAudit excludes data and metadata from ordinary tools, including
// aliases of the reserved store or a hard-linked store object.
func (r *RootProtection) CheckAudit(path string, info, parent os.FileInfo) error {
	store := r.config.audit
	if store == "" {
		return nil
	}
	if isInsideRoot(store, path) {
		return ErrAuditPath
	}
	storeInfo, err := PathIdentity(store)
	if err != nil || !storeInfo.IsDir() {
		// The selected namespace remains reserved. An unavailable store
		// must not become a global denial in Operation First.
		return nil
	}
	if sameIdentity(storeInfo, info) || sameIdentity(storeInfo, parent) {
		return ErrAuditPath
	}
	if err := checkAuditAncestors(path, parent, storeInfo); err != nil {
		return err
	}
	if info == nil || !info.Mode().IsRegular() {
		return nil
	}
	multiple, err := multipleLinks(path, info)
	if err != nil {
		return ErrAuditPath
	}
	if !multiple {
		return nil
	}
	root, err := os.OpenRoot(store)
	if err != nil {
		return ErrAuditPath
	}
	defer root.Close()
	remaining := AuditTreeMaxEntries
	return checkAuditIdentity(root, ".", 0, info, &remaining)
}

func checkAuditIdentity(root *os.Root, name string, depth int, target os.FileInfo, remaining *int) error {
	if depth > 3 {
		return ErrAuditPath
	}
	dir, err := root.Open(name)
	if err != nil {
		return ErrAuditPath
	}
	defer dir.Close()
	items, err := dir.ReadDir(*remaining + 1)
	if err != nil && err != io.EOF || len(items) > *remaining {
		return ErrAuditPath
	}
	*remaining -= len(items)
	for _, item := range items {
		path := filepath.Join(name, item.Name())
		if item.IsDir() {
			if err := checkAuditIdentity(root, path, depth+1, target, remaining); err != nil {
				return err
			}
		} else if item.Type().IsRegular() {
			candidate, err := PathIdentity(filepath.Join(root.Name(), path))
			if err != nil || sameIdentity(candidate, target) {
				return ErrAuditPath
			}
		}
	}
	return nil
}

func checkAuditAncestors(path string, parent, root os.FileInfo) error {
	if parent == nil {
		return nil
	}
	for dir := filepath.Dir(filepath.Dir(path)); filepath.Dir(dir) != dir; dir = filepath.Dir(dir) {
		ancestor, err := PathIdentity(dir)
		if err != nil || sameIdentity(ancestor, root) {
			return ErrAuditPath
		}
	}
	return nil
}
