package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestAppendCreatesAndExtendsFile(t *testing.T) {
	root := t.TempDir()
	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := filesystem.Append("log.txt", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.Append("log.txt", []byte(" second")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "log.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "first second" {
		t.Fatalf("content=%q", content)
	}
}

func TestAppendEnforcesPayloadLimit(t *testing.T) {
	filesystem, err := NewLocalFileSystemWithPolicyAndLimits(t.TempDir(), security.DefaultPolicy(), Limits{
		MaxWriteBytes: 2, MaxListEntries: 1, MaxCopyBytes: 1, MaxDeleteEntries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := filesystem.Append("log.txt", []byte("abc")); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("error=%v, want ErrLimitExceeded", err)
	}
}

func TestAppendRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := filesystem.Append(".", []byte("x")); !errors.Is(err, ErrPathIsDirectory) {
		t.Fatalf("error=%v, want ErrPathIsDirectory", err)
	}
}
