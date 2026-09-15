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

func processDetails(ctx context.Context, pid uint32) (Details, error) {
	if err := ctx.Err(); err != nil {
		return Details{}, err
	}
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "stat"))
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return Details{}, ErrNotFound
		case errors.Is(err, os.ErrPermission):
			return Details{}, ErrAccessDenied
		default:
			return Details{}, err
		}
	}
	return parseProcStat(pid, string(raw))
}

func parseProcStat(expectedPID uint32, value string) (Details, error) {
	open := strings.IndexByte(value, '(')
	close := strings.LastIndex(value, ") ")
	if open < 1 || close <= open+1 {
		return Details{}, errors.New("invalid proc stat")
	}
	parsedPID, err := strconv.ParseUint(strings.TrimSpace(value[:open]), 10, 32)
	if err != nil || uint32(parsedPID) != expectedPID {
		return Details{}, errors.New("invalid proc stat pid")
	}
	fields := strings.Fields(value[close+2:])
	// fields begins with field 3 (state); ppid is field 4 and num_threads is
	// field 20 in proc_pid_stat(5).
	if len(fields) <= 17 {
		return Details{}, errors.New("short proc stat")
	}
	parentPID, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return Details{}, errors.New("invalid proc stat parent pid")
	}
	threadCount, err := strconv.ParseUint(fields[17], 10, 32)
	if err != nil || threadCount == 0 {
		return Details{}, errors.New("invalid proc stat thread count")
	}
	return Details{
		PID: expectedPID, Name: value[open+1 : close], ParentPID: uint32(parentPID), ThreadCount: uint32(threadCount),
	}, nil
}
