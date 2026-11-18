package fs

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func typedFilesystem(t *testing.T, root string, limits Limits) *LocalFileSystem {
	t.Helper()
	types, err := config.NewFileTypePolicy([]string{".txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	f, err := NewLocalFileSystemWithPolicyAndLimits(root, security.Policy{AllowHiddenFiles: true, FollowSymlinks: true}, limits, types)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFileTypesEightOperations(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	writeTestFile(t, filepath.Join(root, "keep.txt"), "allowed")
	writeTestFile(t, filepath.Join(root, "denied.exe"), "denied-marker")
	writeTestFile(t, filepath.Join(root, ".env"), "hidden")
	if made, err := f.Mkdir("directory.exe/deep.bin"); err != nil || !made {
		t.Fatal("structural directory", made, err)
	}
	entries, err := f.List(".")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name)
	}
	if !slices.Equal(names, []string{"directory.exe", "keep.txt"}) {
		t.Fatal(names)
	}
	if b, err := f.Read("keep.txt", 100); err != nil || string(b) != "allowed" {
		t.Fatal(string(b), err)
	}
	if _, err := f.Stat("keep.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stat("missing.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for _, name := range []string{"denied.exe", ".env"} {
		if b, err := f.Read(name, 100); !errors.Is(err, ErrFileTypeDenied) || len(b) != 0 {
			t.Fatal(name, string(b), err)
		}
		if _, err := f.Stat(name); !errors.Is(err, ErrFileTypeDenied) || errors.Is(err, ErrNotFound) {
			t.Fatal(name, err)
		}
	}
	for _, operation := range []func() error{
		func() error { return f.Write("new.exe", []byte("new"), false) },
		func() error { return f.Write("denied.exe", []byte("new"), true) },
		func() error { return f.Copy("denied.exe", "keep.txt", true) },
		func() error { return f.Copy("keep.txt", "denied.exe", true) },
		func() error { return f.Copy("keep.txt", "new.exe", false) },
		func() error { return f.Move("denied.exe", "new.txt", false) },
		func() error { return f.Move("keep.txt", "denied.exe", true) },
		func() error { return f.Move("keep.txt", "new.exe", false) },
		func() error { return f.Delete("denied.exe", false) },
	} {
		if err := operation(); !errors.Is(err, ErrFileTypeDenied) || !errors.Is(err, os.ErrPermission) {
			t.Fatal(err)
		}
		for name, want := range map[string]string{"keep.txt": "allowed", "denied.exe": "denied-marker"} {
			if b, e := os.ReadFile(filepath.Join(root, name)); e != nil || string(b) != want {
				t.Fatal("effect before denial", name, string(b), e)
			}
		}
		for _, name := range []string{"new.exe", "new.txt"} {
			if _, e := os.Lstat(filepath.Join(root, name)); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("created denied output", name, e)
			}
		}
	}
	if err := f.Write("write.TXT", []byte("write"), false); err != nil {
		t.Fatal(err)
	}
	if err := f.Copy("keep.txt", "copy.txt", false); err != nil {
		t.Fatal(err)
	}
	if err := f.Move("copy.txt", "renamed.TXT", false); err != nil {
		t.Fatal(err)
	}
	if err := f.Delete("renamed.TXT", false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Mkdir("denied.exe"); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
	if err := f.Copy("directory.exe", "dircopy.txt", false); !errors.Is(err, ErrCopyDirectoryUnsupported) {
		t.Fatal(err)
	}
}

func TestFileTypeTreePreflightAndResources(t *testing.T) {
	for _, operation := range []string{"delete", "move"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			f := typedFilesystem(t, root, DefaultLimits())
			mkdir(t, filepath.Join(root, "tree", "deep"))
			writeTestFile(t, filepath.Join(root, "tree", "good.txt"), "retain")
			writeTestFile(t, filepath.Join(root, "tree", "deep", "bad.exe"), "retain-bad")
			var err error
			if operation == "delete" {
				err = f.Delete("tree", true)
			} else {
				err = f.Move("tree", "target", false)
			}
			if !errors.Is(err, ErrFileTypeDenied) {
				t.Fatal(err)
			}
			for name, want := range map[string]string{"tree/good.txt": "retain", "tree/deep/bad.exe": "retain-bad"} {
				b, e := os.ReadFile(filepath.Join(root, name))
				if e != nil || string(b) != want {
					t.Fatal("partial tree effect", name, e)
				}
			}
			if _, e := os.Stat(filepath.Join(root, "target")); !errors.Is(e, os.ErrNotExist) {
				t.Fatal(e)
			}
			if e := os.Remove(filepath.Join(root, "tree", "deep", "bad.exe")); e != nil {
				t.Fatal(e)
			}
			if err := f.Move("tree", "allowed-directory.exe", false); err != nil {
				t.Fatal(err)
			}
			if err := f.Delete("allowed-directory.exe", true); err != nil {
				t.Fatal(err)
			}
		})
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "filtered.exe"), "x")
	l := DefaultLimits()
	l.MaxScanSize = 2
	f := typedFilesystem(t, root, l)
	if _, err := f.List("."); !errors.Is(err, ErrLimitExceeded) {
		t.Fatal("filtered entries must consume scan budget", err)
	}
	mkdir(t, filepath.Join(root, "tree"))
	writeTestFile(t, filepath.Join(root, "tree", "long.txt"), "x")
	if err := f.Delete("tree", true); !errors.Is(err, ErrLimitExceeded) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "tree", "long.txt")); err != nil {
		t.Fatal("resource denial changed tree", err)
	}
}

func TestFileTypesLinksAndDeterministicDrift(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	good := filepath.Join(root, "good.txt")
	bad := filepath.Join(root, "bad.exe")
	alias := filepath.Join(root, "alias.txt")
	writeTestFile(t, good, "good")
	writeTestFile(t, bad, "bad")
	createTestSymlinkOrSkip(t, bad, alias)
	if _, err := f.Read("alias.txt", 100); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal("suffix laundered target", err)
	}
	if _, err := f.Stat("alias.txt"); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
	if entries, err := f.List("."); err != nil || len(entries) != 1 || entries[0].Name != "good.txt" {
		t.Fatal(entries, err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	createTestSymlinkOrSkip(t, good, alias)
	if b, err := f.Read("alias.txt", 100); err != nil || string(b) != "good" {
		t.Fatal(string(b), err)
	}
	expected, err := f.inspectEffect(alias, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	createTestSymlinkOrSkip(t, bad, alias)
	if err := f.recheckEffect(expected, true); err == nil {
		t.Fatal("late swap admitted")
	}
	if _, err := f.openDestination(expected, 0600, nil); err == nil {
		t.Fatal("late destination swap admitted")
	}
	handle, err := os.Open(bad)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := f.checkOpened(handle, expected, false); err == nil {
		t.Fatal("different opened identity admitted")
	}
	if b, err := os.ReadFile(bad); err != nil || string(b) != "bad" {
		t.Fatal("late swap damaged target", err)
	}
}

func TestFileTypeProjectionAndUnsupportedBinding(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	mkdir(t, filepath.Join(root, "one"))
	mkdir(t, filepath.Join(root, "two"))
	writeTestFile(t, filepath.Join(root, "one", "target.txt"), "safe")
	link := filepath.Join(root, "one", "alias.txt")
	createTestSymlinkOrSkip(t, "target.txt", link)
	if err := f.Move("one/alias.txt", "two/alias.txt", false); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal("unproved relative projection", err)
	}
	if err := f.Move("one", "moved", false); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal("unproved tree projection", err)
	}
	if err := f.Move("one/alias.txt", "one/renamed.txt", false); err != nil {
		t.Fatal("same-parent link rename", err)
	}
	createTestSymlinkOrSkip(t, "one", filepath.Join(root, "relative-directory"))
	if err := f.Move("relative-directory", "two/relocated", false); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal("directory link projection was not established", err)
	}
	mkdir(t, filepath.Join(root, "absolute-tree"))
	createTestSymlinkOrSkip(t, filepath.Join(root, "one", "target.txt"), filepath.Join(root, "absolute-tree", "absolute.txt"))
	if err := f.Move("absolute-tree", "absolute-moved", false); err != nil {
		t.Fatal("proved absolute link projection", err)
	}
	if _, err := NewProtectedRootFileSystems(nil, nil, config.FileTypePolicy{}); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal("missing backend binding", err)
	}
	if _, err := NewLocalFileSystemWithGuard(nil, DefaultLimits(), config.FileTypePolicy{}, config.FileTypePolicy{}); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
	if _, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), DefaultLimits(), config.FileTypePolicy{}, config.FileTypePolicy{}); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.checkFileType(effectPath{path: filepath.Join(root, "special.txt"), info: specialTypeInfo{info}}, false); !errors.Is(err, ErrFileTypeDenied) {
		t.Fatal(err)
	}
}

func TestFileTypeUnsafeSpellingsBeforeNativeLookup(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	// Move classifies the source before applying portable rules to its target:
	// a directory target is structural, while this real file target is not.
	writeTestFile(t, filepath.Join(root, "ordinary.txt"), "retain")
	for _, name := range []string{"nul\x00.txt", "control\n.txt", "\xff.txt", "file:stream.txt", "tail.txt.", "tail.txt "} {
		if err := f.Write(name, nil, false); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
		if _, err := f.Read(name, 1); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
		if _, err := f.Stat(name); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
		if err := f.Copy("ordinary.txt", name, false); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
		if err := f.Move("ordinary.txt", name, false); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
		if err := f.Delete(name, false); !errors.Is(err, ErrFileTypeDenied) {
			t.Fatal(name, err)
		}
	}
}

func TestFileTypesListDirectoryLinksRemainStructural(t *testing.T) {
	root := t.TempDir()
	f := typedFilesystem(t, root, DefaultLimits())
	target := filepath.Join(root, "target")
	mkdir(t, target)
	writeTestFile(t, filepath.Join(target, "allowed.txt"), "safe")
	createTestSymlinkOrSkip(t, target, filepath.Join(root, "directory-alias.exe"))
	entries, err := f.List(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	if !slices.Equal(names, []string{"directory-alias.exe", "target"}) {
		t.Fatal("directory link was classified by its suffix", names)
	}
	if info, err := f.Stat("directory-alias.exe"); err != nil || !info.IsDir {
		t.Fatal("directory link metadata", info, err)
	}
	if children, err := f.List("directory-alias.exe"); err != nil || len(children) != 1 || children[0].Name != "allowed.txt" {
		t.Fatal("structural traversal", children, err)
	}
}

type specialTypeInfo struct{ os.FileInfo }

func (specialTypeInfo) IsDir() bool       { return false }
func (specialTypeInfo) Mode() os.FileMode { return os.ModeDevice }
