//go:build !windows

package fs

import (
	"fmt"
	"os"
)

func portablePermissions(mode os.FileMode) string {
	return fmt.Sprintf("%04o", mode.Perm())
}
