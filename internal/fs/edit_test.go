package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestEditRangeReplacesInsertsAndDeletesBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	size, err := f.EditRange("a.txt", 2, 4, []byte("XYZ"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "abXYZef" || size != 7 {
		t.Fatalf("got %q size %d", got, size)
	}
	if _, err := f.EditRange("a.txt", 99, 99, nil); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected limit error, got %v", err)
	}
}

func TestEditRangeEnforcesTypeAndSizeLimits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	limits := DefaultLimits()
	limits.MaxWriteBytes = 4
	f, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "full.txt"), []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.EditRange("full.txt", 4, 4, []byte("5")); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected result limit error, got %v", err)
	}
	if _, err := f.EditRange(".", 0, 0, nil); !errors.Is(err, ErrPathIsDirectory) {
		t.Fatalf("expected directory error, got %v", err)
	}
}
