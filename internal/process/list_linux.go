//go:build linux

package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func listProcesses(ctx context.Context) ([]Entry, error) {
	directories, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(directories))
	for _, directory := range directories {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !directory.IsDir() {
			continue
		}
		pid, err := strconv.ParseUint(directory.Name(), 10, 32)
		if err != nil || pid == 0 {
			continue
		}
		name, err := os.ReadFile(filepath.Join("/proc", directory.Name(), "comm"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
				continue
			}
			continue
		}
		cleanName := strings.TrimSpace(string(name))
		if cleanName == "" {
			continue
		}
		entries = append(entries, Entry{PID: uint32(pid), Name: cleanName})
	}
	return entries, nil
}
