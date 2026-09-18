//go:build !windows

package fs

import "os"

func publishAtomicWrite(stage string, target string, overwrite bool) error {
	if overwrite {
		return os.Rename(stage, target)
	}
	if err := os.Link(stage, target); err != nil {
		return err
	}
	return os.Remove(stage)
}
