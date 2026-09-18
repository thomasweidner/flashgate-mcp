//go:build windows

package fs

import "os"

func portablePermissions(os.FileMode) string {
	return ""
}
