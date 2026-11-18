//go:build linux

package roots

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestFileTypesNativeLinuxUnrestrictedCompatibility(t *testing.T) {
	for _, mode := range []struct{ name, fields string }{
		{"absent", ""},
		{"unrestricted default", "allowed_extensions='unrestricted'\n"},
		{"unrestricted true", "allowed_extensions='unrestricted'\nallow_extensionless=true\n"},
		{"root reset", "policy='p'\nallowed_extensions='unrestricted'\nallow_extensionless=true\n[policy.p]\nallowed_extensions=['.txt']\nallow_extensionless=false\n"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "config.toml")
			r := loadRegistry(t, source, "[files]\ndefault_id='work'\n"+
				rootTable(t, "work", root)+"access=['read','write']\n"+mode.fields)
			op := func(name string) fs.Operand { return fs.Operand{RootID: "work", Path: name} }
			assertBytes := func(t *testing.T, name, want string) {
				t.Helper()
				b, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(b) != want {
					t.Fatalf("%q: %q, %v; want %q", name, b, err, want)
				}
			}
			assertAbsent := func(t *testing.T, name string) {
				t.Helper()
				if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%q still exists: %v", name, err)
				}
			}
			for _, name := range []string{"normal.txt", "colon:token.txt", "tail.txt.", "tail.txt ", `back\slash.txt`} {
				t.Run(name, func(t *testing.T) {
					if err := os.WriteFile(filepath.Join(root, name), []byte("before"), 0600); err != nil {
						t.Fatal(err)
					}
					if b, err := r.Read(op(name), 100); err != nil || string(b) != "before" {
						t.Fatal("read", string(b), err)
					}
					if info, err := r.Stat(op(name)); err != nil || info.Name != name || info.IsDir {
						t.Fatal("stat", info, err)
					}
					entries, err := r.List(op("."))
					if err != nil || !slices.ContainsFunc(entries, func(e fs.Entry) bool { return e.Name == name }) {
						t.Fatal("list omitted native name", name, entries, err)
					}
					if err := r.Write(op(name), []byte("after"), true); err != nil {
						t.Fatal("overwrite", err)
					}
					assertBytes(t, name, "after")
					created := "new-" + name
					if err := r.Write(op(created), []byte("created"), false); err != nil {
						t.Fatal("create", err)
					}
					assertBytes(t, created, "created")
					copyName, movedName := "copy-"+name, "moved-"+name
					if err := r.Copy(op(name), op(copyName), false); err != nil {
						t.Fatal("copy", err)
					}
					if err := r.Copy(op(name), op(copyName), true); err != nil {
						t.Fatal("copy overwrite", err)
					}
					assertBytes(t, copyName, "after")
					if err := r.Move(op(copyName), op(movedName), false); err != nil {
						t.Fatal("move", err)
					}
					assertAbsent(t, copyName)
					assertBytes(t, movedName, "after")
					for _, leaf := range []string{name, created, movedName} {
						if err := r.Delete(op(leaf), false); err != nil {
							t.Fatal("delete", err)
						}
						assertAbsent(t, leaf)
					}
					dir, movedDir := "dir-"+name, "moved-dir-"+name
					if made, err := r.Mkdir(op(dir)); err != nil || !made {
						t.Fatal("mkdir", made, err)
					}
					if info, err := r.Stat(op(dir)); err != nil || !info.IsDir || info.Name != dir {
						t.Fatal("directory stat", info, err)
					}
					if err := r.Write(op(filepath.Join(dir, name)), []byte("child"), false); err != nil {
						t.Fatal("directory child", err)
					}
					if err := r.Move(op(dir), op(movedDir), false); err != nil {
						t.Fatal("directory move", err)
					}
					if children, err := r.List(op(movedDir)); err != nil || len(children) != 1 || children[0].Name != name {
						t.Fatal("directory list", children, err)
					}
					if err := r.Delete(op(movedDir), true); err != nil {
						t.Fatal("recursive delete", err)
					}
					assertAbsent(t, movedDir)
				})
			}
			for _, escape := range []string{"../outside:token.txt", filepath.Join(t.TempDir(), "outside.txt")} {
				if _, err := r.Read(op(escape), 100); !errors.Is(err, security.ErrPathTraversal) && !errors.Is(err, security.ErrAbsolutePath) {
					t.Fatal("root read escape", err)
				}
				if err := r.Write(op(escape), nil, false); !errors.Is(err, security.ErrPathTraversal) && !errors.Is(err, security.ErrAbsolutePath) {
					t.Fatal("root write escape", err)
				}
			}
			alias := "config:alias.txt"
			if err := os.Link(source, filepath.Join(root, alias)); err != nil {
				t.Fatal(err)
			}
			for _, effect := range []func() error{
				func() error { _, err := r.Read(op(alias), 65536); return err },
				func() error { return r.Write(op(alias), nil, true) },
				func() error { return r.Copy(op(alias), op("export.txt"), false) },
				func() error { return r.Move(op(alias), op("renamed.txt"), false) },
				func() error { return r.Delete(op(alias), false) },
			} {
				if err := effect(); !errors.Is(err, security.ErrConfigurationAccess) {
					t.Fatal("native name bypassed configuration protection", err)
				}
			}
			if _, err := r.Stat(op(alias)); err != nil {
				t.Fatal("content-independent metadata", err)
			}
			assertAbsent(t, "export.txt")
			assertAbsent(t, "renamed.txt")
			if err := syscall.Mkfifo(filepath.Join(root, "pipe.txt"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Read(op("pipe.txt"), 100); !errors.Is(err, fs.ErrFileTypeDenied) {
				t.Fatal("unrestricted admitted a special object", err)
			}
		})
	}
}

func TestFileTypesNativeLinuxRestrictiveNamesRemainDenied(t *testing.T) {
	for _, fields := range []string{
		"allowed_extensions=['.txt']\n",
		"allowed_extensions='unrestricted'\nallow_extensionless=false\n",
		"allow_extensionless=false\n",
	} {
		root := t.TempDir()
		r := loadRegistry(t, filepath.Join(root, "config.toml"), "[files]\ndefault_id='work'\n"+
			rootTable(t, "work", root)+"access=['read','write']\n"+fields)
		op := func(name string) fs.Operand { return fs.Operand{RootID: "work", Path: name} }
		if err := r.Write(op("normal.txt"), []byte("allowed"), false); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"colon:token.txt", "tail.txt.", "tail.txt ", `back\slash.txt`} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("retain"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, effect := range []func() error{
				func() error { _, err := r.Read(op(name), 100); return err },
				func() error { _, err := r.Stat(op(name)); return err },
				func() error { return r.Write(op(name), nil, true) },
				func() error { return r.Copy(op(name), op("output.txt"), false) },
				func() error { return r.Copy(op("normal.txt"), op(name), true) },
				func() error { return r.Move(op(name), op("output.txt"), false) },
				func() error { return r.Move(op("normal.txt"), op(name), true) },
				func() error { return r.Delete(op(name), false) },
			} {
				if err := effect(); !errors.Is(err, fs.ErrFileTypeDenied) {
					t.Fatal("restricted native name admitted", name, err)
				}
			}
			if b, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(b) != "retain" {
				t.Fatal("denial changed native file", name, err)
			}
			if entries, err := r.List(op(".")); err != nil || slices.ContainsFunc(entries, func(e fs.Entry) bool { return e.Name == name }) {
				t.Fatal("restricted list exposed native name", entries, err)
			}
		}
		if b, err := os.ReadFile(filepath.Join(root, "normal.txt")); err != nil || string(b) != "allowed" {
			t.Fatal("denial changed allowed source", err)
		}
		if _, err := os.Stat(filepath.Join(root, "output.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("denial created output", err)
		}
	}
}
