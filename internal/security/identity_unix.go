//go:build !windows

package security

import (
	"os"
	"path/filepath"
)

// PathIdentity returns metadata for the current target of path.
func PathIdentity(path string) (os.FileInfo, error) { return os.Stat(path) }

func configPathEqual(left, right string) bool { return filepath.Clean(left) == filepath.Clean(right) }

func configNameEqual(left, right string) bool { return left == right }
