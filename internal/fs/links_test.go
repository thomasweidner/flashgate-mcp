package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func linkFilesystem(t *testing.T, follow bool) (*LocalFileSystem, string) {
	t.Helper()
	root := t.TempDir()
	f, err := NewLocalFileSystemWithPolicy(root, security.Policy{FollowSymlinks: follow})
	if err != nil {
		t.Fatal(err)
	}
	return f, root
}

func TestLinkPolicyNativeEightOperations(t *testing.T) {
	for _, follow := range []bool{false, true} {
		for _, op := range []string{"read", "stat", "list", "write", "mkdir", "delete", "copy-source", "copy-target", "move-source", "move-target"} {
			t.Run(op+map[bool]string{false: "-deny", true: "-allow"}[follow], func(t *testing.T) {
				f, root := linkFilesystem(t, follow)
				mkdir(t, filepath.Join(root, "directory"))
				writeTestFile(t, filepath.Join(root, "target.txt"), "target")
				writeTestFile(t, filepath.Join(root, "source.txt"), "source")
				createTestSymlinkOrSkip(t, "target.txt", filepath.Join(root, "alias.txt"))
				createTestSymlinkOrSkip(t, "directory", filepath.Join(root, "dir-alias"))
				var err error
				switch op {
				case "read":
					var b []byte
					b, err = f.Read("alias.txt", 100)
					if err == nil && string(b) != "target" {
						t.Fatal(string(b))
					}
				case "stat":
					var m Metadata
					m, err = f.Stat("alias.txt")
					if err == nil && (m.Size != 6 || m.Name != "alias.txt") {
						t.Fatal(m)
					}
				case "list":
					_, err = f.List("dir-alias")
				case "write":
					err = f.Write("alias.txt", []byte("replacement"), true)
				case "mkdir":
					_, err = f.Mkdir("dir-alias/new")
				case "delete":
					err = f.Delete("alias.txt", false)
				case "copy-source":
					err = f.Copy("alias.txt", "copy.txt", false)
				case "copy-target":
					err = f.Copy("source.txt", "alias.txt", true)
				case "move-source":
					err = f.Move("alias.txt", "renamed.txt", false)
				case "move-target":
					err = f.Move("source.txt", "alias.txt", true)
				}
				if follow && err != nil {
					t.Fatal(err)
				}
				if !follow && !errors.Is(err, security.ErrSymlinkDenied) {
					t.Fatal("default did not deny", err)
				}
				want := "target"
				if follow && op == "write" {
					want = "replacement"
				}
				if follow && op == "copy-target" {
					want = "source"
				}
				b, err := os.ReadFile(filepath.Join(root, "target.txt"))
				if err != nil || string(b) != want {
					t.Fatal("unexpected target effect", string(b), err)
				}
				if !follow {
					entries, err := f.List(".")
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range entries {
						if strings.Contains(entry.Name, "alias") {
							t.Fatal("denied link listed", entry)
						}
					}
				}
			})
		}
	}
}

func TestLinkPolicyAbsoluteEscapesChainsAndCycles(t *testing.T) {
	f, root := linkFilesystem(t, true)
	writeTestFile(t, filepath.Join(root, "target.txt"), "safe")
	createTestSymlinkOrSkip(t, filepath.Join(root, "target.txt"), filepath.Join(root, "absolute.txt"))
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "outside.txt"), "outside")
	relative, err := filepath.Rel(root, filepath.Join(outside, "outside.txt"))
	if err != nil {
		t.Fatal(err)
	}
	createTestSymlinkOrSkip(t, relative, filepath.Join(root, "escape.txt"))
	createTestSymlinkOrSkip(t, "target.txt", filepath.Join(root, "one.txt"))
	createTestSymlinkOrSkip(t, "one.txt", filepath.Join(root, "two.txt"))
	createTestSymlinkOrSkip(t, "cycle-b.txt", filepath.Join(root, "cycle-a.txt"))
	createTestSymlinkOrSkip(t, "cycle-a.txt", filepath.Join(root, "cycle-b.txt"))
	createTestSymlinkOrSkip(t, "absolute.txt", filepath.Join(root, "absolute-chain.txt"))
	for _, name := range []string{"absolute.txt", "escape.txt", "cycle-a.txt", "absolute-chain.txt"} {
		for _, op := range []func() error{
			func() error { _, err := f.Read(name, 100); return err }, func() error { _, err := f.Stat(name); return err },
			func() error { return f.Write(name, []byte("changed"), true) }, func() error { return f.Copy(name, "output.txt", false) },
			func() error { return f.Copy("target.txt", name, true) }, func() error { return f.Move(name, "moved.txt", false) },
			func() error { return f.Move("target.txt", name, true) }, func() error { return f.Delete(name, false) },
		} {
			if err := op(); err == nil {
				t.Fatal("unsafe link admitted", name)
			}
		}
	}
	if b, err := f.Read("two.txt", 100); err != nil || string(b) != "safe" {
		t.Fatal("relative chain", string(b), err)
	}
	for _, p := range []string{"output.txt", "moved.txt"} {
		if _, err := os.Lstat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Fatal("denial materialized output", err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(outside, "outside.txt")); err != nil || string(b) != "outside" {
		t.Fatal("escape effect", err)
	}
}

func TestLinkPolicyDeterministicProtectedDrift(t *testing.T) {
	for _, op := range []string{"read", "write-open", "write-truncate", "create", "mkdir", "delete", "move-source", "move-target", "copy-source", "copy-target"} {
		t.Run(op, func(t *testing.T) {
			root := t.TempDir()
			config := filepath.Join(root, "config.txt")
			writeTestFile(t, config, "protected-marker")
			loaded, err := security.PathIdentity(config)
			if err != nil {
				t.Fatal(err)
			}
			guard, err := security.NewPathGuardWithPolicy(root, security.Policy{FollowSymlinks: true})
			if err != nil {
				t.Fatal(err)
			}
			protection, err := bindTestConfigProtection(guard, config, loaded)
			if err != nil {
				t.Fatal(err)
			}
			f := mustProtectedFileSystem(t, protection, DefaultLimits())
			writeTestFile(t, filepath.Join(root, "source.txt"), "source-marker")
			writeTestFile(t, filepath.Join(root, "target.txt"), "target-marker")
			stage := "open"
			selected := "source.txt"
			switch op {
			case "write-open":
				selected = "target.txt"
			case "write-truncate":
				stage = "truncate"
				selected = "target.txt"
			case "create", "mkdir":
				selected = "new.txt"
				if op == "mkdir" {
					stage = "mkdir"
				}
			case "delete":
				stage = "remove"
			case "move-source":
				stage = "rename"
			case "move-target":
				stage = "rename"
				selected = "target.txt"
			case "copy-source":
				stage = "truncate"
			case "copy-target":
				stage = "truncate"
				selected = "target.txt"
			}
			intercepted := false
			f.beforeNative = func(at string) {
				if at != stage || intercepted {
					return
				}
				intercepted = true
				p := filepath.Join(root, selected)
				if _, err := os.Lstat(p); err == nil {
					if err := os.Rename(p, p+".previous"); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Link(config, p); err != nil {
					t.Fatal(err)
				}
			}
			switch op {
			case "read":
				_, err = f.Read("source.txt", 100)
			case "write-open", "write-truncate":
				err = f.Write("target.txt", []byte("changed"), true)
			case "create":
				err = f.Write("new.txt", []byte("changed"), false)
			case "mkdir":
				_, err = f.Mkdir("new.txt")
			case "delete":
				err = f.Delete("source.txt", false)
			case "move-source", "move-target":
				err = f.Move("source.txt", "target.txt", true)
			case "copy-source", "copy-target":
				err = f.Copy("source.txt", "target.txt", true)
			}
			if !intercepted || err == nil {
				t.Fatal("late protected drift was not denied", intercepted, err)
			}
			for _, item := range []struct{ name, want string }{{"config.txt", "protected-marker"}, {selected, "protected-marker"}} {
				b, e := os.ReadFile(filepath.Join(root, item.name))
				if e != nil || string(b) != item.want {
					t.Fatal("protected effect", item.name, string(b), e)
				}
			}
			if op != "create" && op != "mkdir" {
				b, e := os.ReadFile(filepath.Join(root, selected+".previous"))
				want := "source-marker"
				if selected == "target.txt" {
					want = "target-marker"
				}
				if e != nil || string(b) != want {
					t.Fatal("premature truncation", string(b), e)
				}
			}
		})
	}
}

func TestLinkPolicyDeterministicParentDrift(t *testing.T) {
	f, root := linkFilesystem(t, true)
	mkdir(t, filepath.Join(root, "parent"))
	outside := t.TempDir()
	blocked := false
	intercepted := false
	f.beforeNative = func(stage string) {
		if stage != "open" || intercepted {
			return
		}
		intercepted = true
		if err := os.Rename(filepath.Join(root, "parent"), filepath.Join(root, "previous")); err != nil {
			blocked = true
			return
		}
		if err := os.Symlink(outside, filepath.Join(root, "parent")); err != nil {
			t.Fatal(err)
		}
	}
	err := f.Write("parent/new.txt", []byte("effect"), false)
	if !intercepted || (!blocked && err == nil) {
		t.Fatal("parent drift admitted", blocked, err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("outside effect", err)
	}
	if blocked {
		t.Log("OS prevented the parent exchange while its handle was held")
	}
}

func TestLinkPolicyRootAliasIdentityAndUnavailableBinding(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	mkdir(t, real)
	alias := filepath.Join(base, "alias")
	createTestSymlinkOrSkip(t, "real", alias)
	writeTestFile(t, filepath.Join(real, "safe.txt"), "safe")
	f, err := NewLocalFileSystemWithPolicy(alias, security.Policy{FollowSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := f.Read("safe.txt", 100); err != nil || string(b) != "safe" {
		t.Fatal(string(b), err)
	}
	if err := os.Rename(alias, alias+".previous"); err != nil {
		t.Fatal(err)
	}
	createTestSymlinkOrSkip(t, "real", alias)
	if _, err := f.Read("safe.txt", 100); err == nil {
		t.Fatal("replacement root link with same target was accepted")
	}
	guard, err := security.NewPathGuard(real)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(real, filepath.Join(base, "previous-real")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalFileSystemWithGuard(guard, DefaultLimits()); err == nil {
		t.Fatal("unavailable native root exposed a filesystem")
	}
}

func TestLinkPolicyRootSelfDeleteHasNoPartialEffects(t *testing.T) {
	f, root := linkFilesystem(t, false)
	writeTestFile(t, filepath.Join(root, "retain.txt"), "retain")
	if err := f.Delete(".", true); err == nil {
		t.Fatal("root self deletion accepted")
	}
	if b, err := os.ReadFile(filepath.Join(root, "retain.txt")); err != nil || string(b) != "retain" {
		t.Fatal("root denial had a partial effect", err)
	}
}

func TestNativeOpenRefusesPrematureTruncation(t *testing.T) {
	f, root := linkFilesystem(t, false)
	p := filepath.Join(root, "retain.txt")
	writeTestFile(t, p, "retain")
	bound, err := f.bindNative(p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer bound.close()
	for _, flags := range []int{os.O_WRONLY | os.O_TRUNC, os.O_WRONLY | os.O_CREATE} {
		file, err := bound.open(flags, 0600)
		if file != nil {
			_ = file.Close()
		}
		if err == nil {
			t.Fatal("destructive native open was accepted")
		}
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "retain" {
		t.Fatal("premature truncation", err)
	}
}

func TestLinkPolicyLeafNamespaceParentsAndDirectoryContents(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, recursive := range []bool{false, true} {
			t.Run(fmt.Sprintf("directory-%v/recursive-%v", directory, recursive), func(t *testing.T) {
				f, root := linkFilesystem(t, true)
				mkdir(t, filepath.Join(root, "links"))
				mkdir(t, filepath.Join(root, "data"))
				writeTestFile(t, filepath.Join(root, "data", "keep.txt"), "retained")
				target := "../data/keep.txt"
				if directory {
					target = "../data"
				}
				createTestSymlinkOrSkip(t, target, filepath.Join(root, "links", "alias.txt"))
				rawTarget, err := os.Readlink(filepath.Join(root, "links", "alias.txt"))
				if err != nil {
					t.Fatal(err)
				}
				if err := f.Move("links/alias.txt", "links/renamed.txt", false); err != nil {
					t.Fatal("same entry-parent rename denied", err)
				}
				if got, err := os.Readlink(filepath.Join(root, "links", "renamed.txt")); err != nil || got != rawTarget {
					t.Fatal("rename changed the relative target", got, err)
				}
				if err := f.Delete("links/renamed.txt", recursive); err != nil {
					t.Fatal("leaf unlink denied", err)
				}
				if _, err := os.Lstat(filepath.Join(root, "links", "renamed.txt")); !os.IsNotExist(err) {
					t.Fatal("leaf entry survived unlink", err)
				}
				if b, err := os.ReadFile(filepath.Join(root, "data", "keep.txt")); err != nil || string(b) != "retained" {
					t.Fatal("namespace effect touched link target contents", string(b), err)
				}
			})
		}
	}
}

func TestLinkPolicyLateTreeProjectionDenied(t *testing.T) {
	for _, target := range []string{"safe.txt", "../config.txt"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			config := filepath.Join(root, "config.txt")
			writeTestFile(t, config, "configuration")
			loaded, err := security.PathIdentity(config)
			if err != nil {
				t.Fatal(err)
			}
			guard, err := security.NewPathGuardWithPolicy(root, security.Policy{FollowSymlinks: true})
			if err != nil {
				t.Fatal(err)
			}
			binding, err := bindTestConfigProtection(guard, config, loaded)
			if err != nil {
				t.Fatal(err)
			}
			f := mustProtectedFileSystem(t, binding, DefaultLimits())
			tree := filepath.Join(root, "tree")
			mkdir(t, tree)
			writeTestFile(t, filepath.Join(tree, "safe.txt"), "safe")
			intercepted := false
			f.beforeNative = func(stage string) {
				if stage == "rename" && !intercepted {
					intercepted = true
					createTestSymlinkOrSkip(t, target, filepath.Join(tree, "alias.txt"))
				}
			}
			if err := f.Move("tree", "moved", false); !intercepted || err == nil {
				t.Fatal("late unproved tree projection admitted", intercepted, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "moved")); !os.IsNotExist(err) {
				t.Fatal("denial produced a destination", err)
			}
			for path, want := range map[string]string{"config.txt": "configuration", "tree/safe.txt": "safe"} {
				if b, err := os.ReadFile(filepath.Join(root, path)); err != nil || string(b) != want {
					t.Fatal("projection denial changed source bytes", path, string(b), err)
				}
			}
		})
	}
}

func TestLinkPolicyTreeRevalidationSharesScanBudget(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "tree"))
	name := "x.txt"
	writeTestFile(t, filepath.Join(root, "tree", name), "retained")
	exact := int64(2 * len(name))
	for _, limit := range []int64{exact - 1, exact} {
		t.Run(fmt.Sprintf("scan-%d", limit), func(t *testing.T) {
			limits := DefaultLimits()
			limits.MaxScanSize = limit
			f, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), limits)
			if err != nil {
				t.Fatal(err)
			}
			err = f.Move("tree", "moved", false)
			if limit < exact {
				if !errors.Is(err, ErrLimitExceeded) {
					t.Fatal("revalidation escaped the aggregate operation scan budget", err)
				}
				if _, err := os.Lstat(filepath.Join(root, "moved")); !os.IsNotExist(err) {
					t.Fatal("budget denial moved the directory", err)
				}
			} else if err != nil {
				t.Fatal("exact aggregate scan budget rejected", err)
			}
		})
	}
}
