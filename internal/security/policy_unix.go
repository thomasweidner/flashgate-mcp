//go:build !windows

package security

import (
	"os"
	"path/filepath"
	"strings"
)

func isUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`)
}

func validatePathMetadata(path string, policy Policy) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if !policy.FollowSymlinks && info.Mode()&os.ModeSymlink != 0 {
		return ErrSymlinkDenied
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_, err := relativeLinkTarget(path)
		return err
	}

	return nil
}

func evalEffectivePath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func nativeLinkTarget(path string) (string, error) { return os.Readlink(path) }

func validateNativeRootMetadata(_ os.FileInfo, _ Policy) error { return nil }

// Hidden-name selection is lexical on these platforms.
func samePolicyMetadata(_, _ os.FileInfo) bool { return true }
