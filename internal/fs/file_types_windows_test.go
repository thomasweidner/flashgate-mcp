//go:build windows

package fs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestFileTypesNativeWindowsAmbiguousNames(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	for _, name := range []string{"new.txt.", "new.txt ", "file:stream.txt", "CON.txt", "aux.txt", "COM1.txt", "LPT².txt", "CONIN$.txt"} {
		if err := f.Write(name, []byte("denied"), false); err == nil {
			t.Fatal("native alias admitted", name)
		}
	}
	writeTestFile(t, filepath.Join(root, "good.txt"), "safe")
	if b, err := f.Read("GOOD.TXT", 100); err != nil || string(b) != "safe" {
		t.Fatal(string(b), err)
	}
}

func TestFileTypesNativeWindowsRestrictiveDirectories(t *testing.T) {
	for _, name := range []string{"native\x7f", "native\u0085"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			f := typedFilesystem(t, root, DefaultLimits())
			if _, err := f.Mkdir(name); err != nil {
				t.Fatal(err)
			}
			if info, err := f.Stat(name); err != nil || !info.IsDir {
				t.Fatal("native-safe structural name", info, err)
			}
			writeTestFile(t, filepath.Join(root, name, "allowed.txt"), "retain")
			writeTestFile(t, filepath.Join(root, name, "denied.exe"), "retain")
			target := "moved-" + name
			if err := f.Move(name, target, false); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("descendant policy lost", err)
			}
			if err := f.Delete(name, true); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("recursive policy lost", err)
			}
			if b, err := os.ReadFile(filepath.Join(root, name, "allowed.txt")); err != nil || string(b) != "retain" {
				t.Fatal("partial effect", string(b), err)
			}
			if err := os.Remove(filepath.Join(root, name, "denied.exe")); err != nil {
				t.Fatal(err)
			}
			if err := f.Move(name, target, false); err != nil {
				t.Fatal("prospective structural target", err)
			}
			if err := f.Delete(target, true); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, name), "regular")
			if _, err := f.Stat(name); !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal("regular file gained directory exemption", err)
			}
		})
	}
}

func TestFileTypesNativeWindowsUnrestrictedUnsafeMappings(t *testing.T) {
	unrestricted, err := config.NewFileTypePolicy(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"absent", "unrestricted"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			var policies []config.FileTypePolicy
			if mode == "unrestricted" {
				policies = []config.FileTypePolicy{unrestricted}
			}
			f, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), DefaultLimits(), policies...)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "good.txt"), "retain")
			for _, name := range []string{"new.txt.", "new.txt ", " leading.txt", "file:stream.txt", "CON.txt", "CON .txt", "COM1 .txt", "aux.txt", "COM1.txt", "LPT².txt", "CONIN$.txt", "nul\x00.txt", "control\x1f.txt", "invalid\xff.txt", "wild*.txt"} {
				if err := f.validateFileOperandSpelling(name); !errors.Is(err, ErrFileTypeDenied) {
					t.Fatal("native mapping admitted before path normalization", name, err)
				}
				for _, effect := range []func() error{
					func() error { _, err := f.Read(name, 100); return err },
					func() error { _, err := f.Stat(name); return err },
					func() error { return f.Write(name, []byte("denied"), true) },
					func() error { return f.Copy(name, "output.txt", false) },
					func() error { return f.Copy("good.txt", name, true) },
					func() error { return f.Move(name, "output.txt", false) },
					func() error { return f.Move("good.txt", name, true) },
					func() error { return f.Delete(name, false) },
				} {
					if err := effect(); !errors.Is(err, ErrFileTypeDenied) {
						t.Fatal("unrestricted bypassed native safety", name, err)
					}
				}
			}
			if b, err := f.Read("GOOD.TXT", 100); err != nil || string(b) != "retain" {
				t.Fatal("normal native access changed", string(b), err)
			}
			if _, err := os.Stat(filepath.Join(root, "output.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("denial created an output", err)
			}
			for _, tc := range []struct{ label, name string }{
				{"DEL", "native\x7f.txt"},
				{"C1", "native\u0085.txt"},
			} {
				t.Run(tc.label, func(t *testing.T) {
					if err := f.Write(tc.name, []byte("native"), false); err != nil {
						t.Fatal("unrestricted imposed portable control rules", err)
					}
					if b, err := f.Read(tc.name, 100); err != nil || string(b) != "native" {
						t.Fatal(string(b), err)
					}
					if info, err := f.Stat(tc.name); err != nil || info.Name != tc.name {
						t.Fatal("native spelling changed", info, err)
					}
					if entries, err := f.List("."); err != nil || !slices.ContainsFunc(entries, func(e Entry) bool { return e.Name == tc.name }) {
						t.Fatal("native name omitted", entries, err)
					}
					strict := typedFilesystem(t, root, DefaultLimits())
					if b, err := strict.Read(tc.name, 100); !errors.Is(err, ErrFileTypeDenied) || len(b) != 0 {
						t.Fatal("restrictive portable rule lost", string(b), err)
					}
					copyName, movedName := "copy-"+tc.name, "moved-"+tc.name
					if err := f.Copy(tc.name, copyName, false); err != nil {
						t.Fatal(err)
					}
					if err := f.Move(copyName, movedName, false); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{tc.name, movedName} {
						if err := f.Delete(name, false); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func TestFileTypesNativeWindowsShortName(t *testing.T) {
	root := t.TempDir()
	long := filepath.Join(root, "long-file-name.documentextension")
	writeTestFile(t, long, "denied-marker")
	p, err := syscall.UTF16PtrFromString(long)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n >= uint32(len(buf)) {
		t.Skipf("short-name API unavailable: %v", err)
	}
	short := syscall.UTF16ToString(buf[:n])
	if strings.EqualFold(filepath.Base(long), filepath.Base(short)) {
		t.Skip("native filesystem has no 8.3 alias")
	}
	ext := strings.ToLower(filepath.Ext(short))
	types, err := config.NewFileTypePolicy([]string{ext}, false)
	if err != nil {
		t.Fatal(err)
	}
	f, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), DefaultLimits(), types)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := f.Read(filepath.Base(short), 100); !errors.Is(err, ErrFileTypeDenied) || len(b) != 0 {
		t.Fatal("short suffix laundered native name", string(b), err)
	}
}

func TestFileTypesNativeWindowsJunction(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	mkdir(t, target)
	writeTestFile(t, filepath.Join(target, "good.txt"), "safe")
	junction := filepath.Join(root, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("junction unavailable: %v (%s)", err, out)
	}
	f := typedFilesystem(t, root, DefaultLimits())
	if _, err := f.Read("junction/good.txt", 100); !errors.Is(err, security.ErrReparsePointDenied) {
		t.Fatal("extension overrode reparse guard", err)
	}
	if _, err := f.BindDirectory("junction"); !errors.Is(err, security.ErrReparsePointDenied) {
		t.Fatal("working-directory selection bypassed the reparse guard", err)
	}
	if _, err := f.BindDirectory("target"); err != nil {
		t.Fatal("ordinary sibling directory was denied", err)
	}
	if _, err := os.Stat(filepath.Join(target, "good.txt")); err != nil {
		t.Fatal(err)
	}
}
