//go:build !windows

package security

import (
	"os"
	"path/filepath"
)

// PathIdentity returns metadata for the current target of path.
func PathIdentity(path string) (os.FileInfo, error) { return os.Stat(path) }

// PathEntryIdentity returns metadata for the directory entry itself.
func PathEntryIdentity(path string) (os.FileInfo, error) { return os.Lstat(path) }

func configPathEqual(left, right string) bool { return filepath.Clean(left) == filepath.Clean(right) }

func configNameEqual(left, right string) bool { return left == right }
