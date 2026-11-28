//go:build !linux && !windows

package security

import "os"

func multipleLinks(string, os.FileInfo) (bool, error) { return false, ErrAuditPath }
