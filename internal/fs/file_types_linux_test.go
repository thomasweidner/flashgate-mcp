//go:build linux

package fs

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFileTypesNativeLinuxSpecialObjectsAndNames(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	pipe := filepath.Join(root, "pipe.txt")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read("pipe.txt", 100); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal("special object admitted", err)
	}
	if _, err := f.Stat("pipe.txt"); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
	if err := f.Write("pipe.txt", nil, true); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
	for _, name := range []string{"space.txt ", "dot.txt.", "stream:payload.txt", "lookalike.txт"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Read(name, 100); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
	}
	if entries, err := f.List("."); err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
