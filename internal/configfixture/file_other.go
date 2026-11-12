//go:build !windows

package configfixture

import (
	"errors"
	"os"
	"path/filepath"
)

// Write creates a new configuration fixture with owner-only permissions.
func Write(path string, data []byte) error {
	parent := filepath.Dir(path)
	expected, err := os.Lstat(parent)
	if err != nil || !expected.IsDir() {
		return errors.New("configuration fixture parent is unavailable")
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return errors.New("configuration fixture parent is aliased")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return errors.New("opening configuration fixture parent failed")
	}
	defer root.Close()
	current, err := root.Stat(".")
	if err != nil || !os.SameFile(expected, current) {
		return errors.New("configuration fixture parent changed")
	}
	file, err := root.OpenFile(filepath.Base(path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("creating configuration fixture failed")
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return errors.New("writing configuration fixture failed")
	}
	return nil
}
