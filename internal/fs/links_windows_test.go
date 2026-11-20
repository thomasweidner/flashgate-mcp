//go:build windows

package fs

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestLinkPolicyWindowsStructuralHiddenDescendants(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "tree", ".internal", "hidden.txt")
	if err := os.MkdirAll(filepath.Dir(child), 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, child, "retained")
	for _, path := range []string{filepath.Dir(child), child} {
		pointer, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		attributes, err := syscall.GetFileAttributes(pointer)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.SetFileAttributes(pointer, attributes|syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
			t.Fatal(err)
		}
	}
	f, err := NewLocalFileSystemWithPolicy(root, security.Policy{FollowSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Move("tree", "moved", false); err != nil {
		t.Fatal("structural move denied an implicit hidden descendant", err)
	}
	if data, err := f.Read("moved/.internal/hidden.txt", 64); !errors.Is(err, security.ErrHiddenPathDenied) || len(data) != 0 {
		t.Fatal("structural handling granted hidden client selection", len(data), err)
	}
	if err := f.Delete("moved", true); err != nil {
		t.Fatal("structural delete denied an implicit hidden descendant", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "moved")); !os.IsNotExist(err) {
		t.Fatal("structural delete was incomplete", err)
	}
}

func TestLinkPolicyWindowsHiddenMetadataDrift(t *testing.T) {
	for _, protected := range []bool{false, true} {
		for _, selected := range []string{"file", "parent", "ancestor", "root"} {
			name := selected
			if protected {
				name += "-protected"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				ancestor := filepath.Join(root, "ancestor")
				parent := filepath.Join(ancestor, "parent")
				if err := os.MkdirAll(parent, 0700); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(parent, "data.txt")
				writeTestFile(t, file, "retained")
				guard, err := security.NewPathGuardWithPolicy(root, security.Policy{FollowSymlinks: true})
				if err != nil {
					t.Fatal(err)
				}
				f, err := NewLocalFileSystemWithGuard(guard, DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				if protected {
					config := filepath.Join(root, "config.txt")
					writeTestFile(t, config, "configuration")
					loaded, err := security.PathIdentity(config)
					if err != nil {
						t.Fatal(err)
					}
					binding, err := bindTestConfigProtection(guard, config, loaded)
					if err != nil {
						t.Fatal(err)
					}
					f = mustProtectedFileSystem(t, binding, DefaultLimits())
				}
				if data, err := f.Read("ancestor/parent/data.txt", 64); err != nil || string(data) != "retained" {
					t.Fatal("fixture was not initially authorized", err)
				}
				target := file
				if selected == "parent" {
					target = parent
				} else if selected == "ancestor" {
					target = ancestor
				} else if selected == "root" {
					target = root
				}
				pointer, err := syscall.UTF16PtrFromString(target)
				if err != nil {
					t.Fatal(err)
				}
				attributes, err := syscall.GetFileAttributes(pointer)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := syscall.SetFileAttributes(pointer, attributes); err != nil {
						t.Error("fixture attribute restoration failed", err)
					}
				}()
				intercepted := false
				f.beforeNative = func(stage string) {
					if stage == "open" && !intercepted {
						intercepted = true
						if err := syscall.SetFileAttributes(pointer, attributes|syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
							t.Fatal(err)
						}
					}
				}
				data, err := f.Read("ancestor/parent/data.txt", 64)
				if !intercepted || err == nil || len(data) != 0 {
					t.Fatal("late hidden-attribute drift admitted", intercepted, len(data), err)
				}
			})
		}
	}
}

func TestLinkPolicyNativeUnknownReparseDenied(t *testing.T) {
	f, root := linkFilesystem(t, true)
	path := filepath.Join(root, "unknown.txt")
	writeTestFile(t, path, "")
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.CloseHandle(h); err != nil {
			t.Error(err)
		}
	}()
	var buffer [24]byte // REPARSE_GUID_DATA_BUFFER, no payload.
	binary.LittleEndian.PutUint32(buffer[:4], 0x42)
	buffer[8] = 0x71
	buffer[23] = 0x19
	var returned uint32
	if err := syscall.DeviceIoControl(h, 0x000900a4, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		t.Skipf("native unknown reparse fixture unavailable: %v; NOT PASS", err)
	}
	defer func() {
		if err := syscall.DeviceIoControl(h, 0x000900ac, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
			t.Error(err)
		}
	}()
	for _, op := range []func() error{
		func() error { _, err := f.Read("unknown.txt", 100); return err }, func() error { _, err := f.Stat("unknown.txt"); return err },
		func() error { return f.Write("unknown.txt", nil, true) }, func() error { return f.Delete("unknown.txt", false) },
		func() error { return f.Copy("unknown.txt", "output.txt", false) }, func() error { return f.Move("unknown.txt", "moved.txt", false) },
	} {
		if err := op(); err == nil {
			t.Fatal("unknown reparse object admitted")
		}
	}
	entries, err := f.List(".")
	if err != nil || len(entries) != 0 {
		t.Fatal("unknown reparse exposed", entries, err)
	}
	for _, p := range []string{"output.txt", "moved.txt"} {
		if _, err := os.Lstat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Fatal("denied reparse effect", p, err)
		}
	}
}
