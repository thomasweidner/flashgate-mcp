//go:build !windows

package main

import (
	"fmt"
	"os"
	"time"
)

func platformStageRenameOps() stageRenameOps {
	return stageRenameOps{os.Rename, time.Now, time.Sleep, func(error) bool { return false }, 1, 0}
}

func stageRenameDiagnostic(err error) error { return err }

func stageDirectory(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("not a real directory: %q", path)
	}
	return info, nil
}
