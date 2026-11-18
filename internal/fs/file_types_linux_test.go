//go:build linux

package fs

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func TestFileTypesRestrictiveDirectorySpellings(t *testing.T) {
	for _, name := range []string{"notes:archive", "tail.", "tail ", `back\slash`} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			f := typedFilesystem(t, root, DefaultLimits())
			if made, err := f.Mkdir(name); err != nil || !made {
				t.Fatal("structural mkdir", made, err)
			}
			for _, spelling := range []string{name, name + "/."} {
				if info, err := f.Stat(spelling); err != nil || !info.IsDir {
					t.Fatal("equivalent structural stat", spelling, info, err)
				}
				if _, err := f.Read(spelling, 100); !errors.Is(err, ErrPathIsDirectory) {
					t.Fatal("directory classified as a file", spelling, err)
				}
			}
			if entries, err := f.List("."); err != nil || !slices.ContainsFunc(entries, func(e Entry) bool { return e.Name == name && e.IsDir }) {
				t.Fatal("structural list", entries, err)
			}
			if err := f.Copy(name, "copy.txt", false); !errors.Is(err, ErrCopyDirectoryUnsupported) {
				t.Fatal("directory-copy contract", err)
			}
			if err := f.Write(filepath.Join(name, "allowed.txt"), []byte("retain"), false); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, name, "denied.exe"), "retain-denied")
			target := "moved-" + name
			if err := f.Move(name, target, false); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("directory exemption bypassed descendant preflight", err)
			}
			if err := f.Delete(name, true); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("recursive directory exemption bypassed file policy", err)
			}
			if b, err := os.ReadFile(filepath.Join(root, name, "allowed.txt")); err != nil || string(b) != "retain" {
				t.Fatal("partial directory effect", string(b), err)
			}
			if _, err := os.Stat(filepath.Join(root, target)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("denied projected directory was created", err)
			}
			if err := os.Remove(filepath.Join(root, name, "denied.exe")); err != nil {
				t.Fatal(err)
			}
			if err := f.Move(name, target, false); err != nil {
				t.Fatal("structural rename to a new native name", err)
			}
			if err := f.Delete(target, true); err != nil {
				t.Fatal("structural recursive delete", err)
			}
			for _, spelling := range []string{name, name + "/."} {
				if _, err := f.Mkdir(name); err != nil {
					t.Fatal(err)
				}
				if err := f.Delete(spelling, false); err != nil {
					t.Fatal("equivalent empty-directory delete", spelling, err)
				}
			}
			writeTestFile(t, filepath.Join(root, name), "regular-file")
			if _, err := f.Stat(name); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("same spelling of a regular file was admitted", err)
			}
			if err := f.Delete(name, false); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("regular file deletion bypassed policy", err)
			}
		})
	}
}
