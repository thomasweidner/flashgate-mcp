//go:build linux

package security

import (
	"os"
	"syscall"
)

func multipleLinks(_ string, info os.FileInfo) (bool, error) {
	node, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, ErrAuditPath
	}
	return node.Nlink > 1, nil
}
