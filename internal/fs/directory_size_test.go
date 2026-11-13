package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestDirectorySizeCountsVisibleEntriesAndReportsProgress(t *testing.T) {
	root := t.TempDir()
	mustWriteDirectorySizeFile(t, filepath.Join(root, "one.txt"), "1234")
	mustWriteDirectorySizeFile(t, filepath.Join(root, "nested", "two.txt"), "567")
	mustWriteDirectorySizeFile(t, filepath.Join(root, ".hidden"), "not counted")

	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	var progress []DirectorySizeProgress
	result, err := filesystem.DirectorySize(context.Background(), ".", func(current DirectorySizeProgress) {
		progress = append(progress, current)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != (DirectorySizeResult{Bytes: 7, Files: 2, Directories: 1, Entries: 3}) {
		t.Fatalf("DirectorySize() = %#v", result)
	}
	if len(progress) != result.Entries || progress[len(progress)-1] != result {
		t.Fatalf("progress = %#v", progress)
	}
}

func TestDirectorySizeEnforcesEntryAndByteLimits(t *testing.T) {
	root := t.TempDir()
	mustWriteDirectorySizeFile(t, filepath.Join(root, "one.txt"), "1234")
	mustWriteDirectorySizeFile(t, filepath.Join(root, "two.txt"), "5678")

	filesystem, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, limits := range []struct {
		entries int
		bytes   int64
	}{
		{entries: 1, bytes: 100},
		{entries: 10, bytes: 4},
	} {
		if _, err := filesystem.directorySize(context.Background(), ".", nil, limits.entries, limits.bytes); !errors.Is(err, ErrLimitExceeded) {
			t.Fatalf("DirectorySize() error = %v, want ErrLimitExceeded", err)
		}
	}
}

func TestDirectorySizeHonorsCancellationAndRequiresDirectory(t *testing.T) {
	root := t.TempDir()
	mustWriteDirectorySizeFile(t, filepath.Join(root, "file.txt"), "x")
	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := filesystem.DirectorySize(ctx, ".", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled DirectorySize() error = %v", err)
	}
	if _, err := filesystem.DirectorySize(context.Background(), "file.txt", nil); !errors.Is(err, ErrPathIsNotDirectory) {
		t.Fatalf("file DirectorySize() error = %v", err)
	}
}

func mustWriteDirectorySizeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
