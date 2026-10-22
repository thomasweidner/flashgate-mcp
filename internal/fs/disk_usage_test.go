package fs

import (
	"errors"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestDiskUsageIsRootConfinedAndPrivacySafe(t *testing.T) {
	t.Parallel()

	filesystem, err := NewLocalFileSystem(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	usage, err := filesystem.DiskUsage(".")
	if err != nil {
		t.Fatalf("DiskUsage: %v", err)
	}
	if usage.TotalBytes == 0 {
		t.Fatal("expected a positive total capacity")
	}
	if usage.UsedBytes > usage.TotalBytes {
		t.Fatalf("used capacity exceeds total: %#v", usage)
	}
	if usage.AvailableBytes > usage.TotalBytes {
		t.Fatalf("available capacity exceeds total: %#v", usage)
	}

	if _, err := filesystem.DiskUsage("../outside"); !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if _, err := filesystem.DiskUsage("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing-path error, got %v", err)
	}
}

func TestLocalFileSystemImplementsDiskUsageProvider(t *testing.T) {
	t.Parallel()
	var _ DiskUsageProvider = (*LocalFileSystem)(nil)
}
