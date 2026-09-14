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

func TestEditMatchesRequiresExactExpectedCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("old / old"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []int{1, 3} {
		if _, err := f.EditMatches("a.txt", []byte("old"), []byte("new"), expected); !errors.Is(err, ErrMatchCountMismatch) {
			t.Fatalf("expected mismatch for expectedMatches=%d, got %v", expected, err)
		}
		unchanged, err := os.ReadFile(path)
		if err != nil || string(unchanged) != "old / old" {
			t.Fatalf("mismatch mutated file: %q, %v", unchanged, err)
		}
	}

	size, err := f.EditMatches("a.txt", []byte("old"), []byte("newer"), 2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "newer / newer" || size != 13 {
		t.Fatalf("got %q size %d err %v", got, size, err)
	}
}

func TestEditMatchesRejectsInvalidAndOversizedInputsWithoutWriting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	limits := DefaultLimits()
	limits.MaxWriteBytes = 4
	f, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), limits)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		old      string
		new      string
		expected int
	}{
		{"", "x", 1},
		{"old", "x", 0},
		{"old", "12345", 1},
	} {
		if _, err := f.EditMatches("a.txt", []byte(tc.old), []byte(tc.new), tc.expected); !errors.Is(err, ErrLimitExceeded) {
			t.Fatalf("expected limit error for %#v, got %v", tc, err)
		}
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != "old" {
		t.Fatalf("invalid input mutated file: %q, %v", unchanged, err)
	}
}
