package fs

import (
	"errors"
	"os"
	"testing"
)

func TestLocalFileSystemReadRejectsVirtualFileScanTruncation(t *testing.T) {
	info, err := os.Stat("/proc/version")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("procfs is not mounted")
	}
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("virtual-file fixture has an unexpected reported size: %d", info.Size())
	}

	limits := DefaultLimits()
	filesystem := mustNewLocalFileSystemWithLimits(t, "/proc", limits)
	complete, err := filesystem.Read("version", 4096)
	if err != nil || len(complete) <= 4 {
		t.Fatalf("read within the scan ceiling: length=%d, error=%v", len(complete), err)
	}

	for _, scan := range []int64{4, int64(len(complete))} {
		limits.MaxScanSize = scan
		filesystem = mustNewLocalFileSystemWithLimits(t, "/proc", limits)
		content, err := filesystem.Read("version", 4096)
		if !errors.Is(err, ErrFileTooLarge) || content != nil {
			t.Fatalf("unproven EOF at scan limit %d: length=%d, error=%v", scan, len(content), err)
		}
	}

	limits.MaxScanSize = int64(len(complete)) + 1
	filesystem = mustNewLocalFileSystemWithLimits(t, "/proc", limits)
	content, err := filesystem.Read("version", 4096)
	if err != nil || string(content) != string(complete) {
		t.Fatalf("proven EOF below the scan ceiling: length=%d, error=%v", len(content), err)
	}
}
