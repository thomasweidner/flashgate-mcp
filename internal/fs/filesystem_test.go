package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestNewLocalFileSystemRejectsEmptyRoot(t *testing.T) {
	t.Parallel()

	_, err := NewLocalFileSystem("")

	if !errors.Is(err, security.ErrEmptyRoot) {
		t.Fatalf("expected ErrEmptyRoot, got %v", err)
	}
}

func bindTestConfigProtection(guard *security.PathGuard, source string, loaded os.FileInfo) (*security.RootProtection, error) {
	protection, err := security.BindConfigProtection(source, loaded)
	if err != nil {
		return nil, err
	}
	return protection.BindRoot(guard, func() error { return nil })
}

func mustProtectedFileSystem(t *testing.T, binding *security.RootProtection, limits Limits) *LocalFileSystem {
	t.Helper()
	filesystems, err := NewProtectedFileSystems([]*security.RootProtection{binding}, limits)
	if err != nil {
		t.Fatal(err)
	}
	return filesystems[0]
}

func TestConfigurationEffectRecheckRejectsIdentityAndParentDrift(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte("protected"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := bindTestConfigProtection(guard, source, loaded)
	if err != nil {
		t.Fatal(err)
	}
	f := mustProtectedFileSystem(t, protection, DefaultLimits())
	parent := filepath.Join(root, "ordinary")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "target")
	if err := os.WriteFile(path, []byte("ordinary"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := f.inspectEffect(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(parent, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, path); err != nil {
		t.Fatal(err)
	}
	if file, err := f.openDestination(expected, 0o600, nil); !errors.Is(err, security.ErrConfigurationAccess) {
		if file != nil {
			_ = file.Close()
		}
		t.Fatalf("substituted leaf reached destructive open: %v", err)
	}
	opened, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	checkErr := f.checkOpened(opened, expected, false)
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkErr, security.ErrConfigurationAccess) {
		t.Fatalf("changed source descriptor reached the read boundary: %v", checkErr)
	}
	missing := filepath.Join(parent, "absent")
	expected, err = f.inspectEffect(missing, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent, filepath.Join(root, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := f.recheckEffect(expected, true); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("substituted parent accepted: %v", err)
	}
	if contents, err := os.ReadFile(source); err != nil || string(contents) != "protected" {
		t.Fatal("protected file changed")
	}
}

func TestConfigurationEffectsHoldSharedLock(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte("protected"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := security.BindConfigProtection(source, loaded)
	if err != nil {
		t.Fatal(err)
	}
	guards := []*security.PathGuard{guard, guard}
	bindings := make([]*security.RootProtection, 0, len(guards))
	var filesystems []*LocalFileSystem
	for i, rootGuard := range guards {
		binding, err := protection.BindRoot(rootGuard, func() error {
			if filesystems != nil {
				other := filesystems[1-i]
				if other.effects.TryLock() {
					other.effects.Unlock()
					t.Error("native effect validation ran without the other root's operation lock")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, binding)
	}
	filesystems, err = NewProtectedFileSystems(bindings, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	var concurrent sync.WaitGroup
	for i := range 8 {
		concurrent.Go(func() {
			path := fmt.Sprintf("sibling-%d", i)
			if err := filesystems[i%2].Write(path, []byte(path), false); err != nil {
				t.Error(err)
			}
			other := filesystems[1-i%2]
			if err := other.Copy("config.toml", path, true); !errors.Is(err, security.ErrConfigurationAccess) {
				t.Errorf("protected configuration copy returned %v", err)
			}
			if contents, err := other.Read(path, 128); err != nil || string(contents) != path {
				t.Errorf("sibling operation did not preserve its bytes: %v", err)
			}
		})
	}
	concurrent.Wait()
}

func TestLinuxMissingConfigurationNamesDenyUnprovedDestinations(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux pathname protection")
	}
	root := t.TempDir()
	parent := filepath.Join(root, "config-dir")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(parent, "config.toml")
	sibling := filepath.Join("config-dir", "sibling")
	ordinary := []byte("ordinary")
	for path, content := range map[string][]byte{
		source: []byte("protected"), filepath.Join(root, sibling): ordinary,
	} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := bindTestConfigProtection(guard, source, loaded)
	if err != nil {
		t.Fatal(err)
	}
	f := mustProtectedFileSystem(t, protection, DefaultLimits())
	if err := f.Write(filepath.Join("config-dir", "new"), ordinary, false); err != nil {
		t.Fatalf("present configuration blocked sibling creation: %v", err)
	}
	renamedSource := filepath.Join(parent, "loaded.toml")
	if err := os.Rename(source, renamedSource); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join("config-dir", "CONFIG.TOML")
	normalizedAlias := filepath.Join("config-dir", "CON\u00adFIG.TOML")
	deny := func(path string, err error) {
		t.Helper()
		if !errors.Is(err, security.ErrConfigurationAccess) {
			t.Errorf("mutation of %q returned %v", path, err)
		}
		if _, err := os.Lstat(filepath.Join(root, path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("mutation left a destination for %q: %v", path, err)
		}
	}
	deny(alias, f.Write(alias, ordinary, false))
	deny(normalizedAlias, f.Copy(sibling, normalizedAlias, false))
	deny(alias, f.Move(sibling, alias, false))
	if err := f.Write(sibling, ordinary, true); err != nil {
		t.Fatalf("missing configuration blocked an existing sibling: %v", err)
	}
	if err := f.Copy(sibling, "new-outside-parent", false); err != nil {
		t.Fatalf("missing configuration blocked a destination in a different parent: %v", err)
	}
	renamedParent := filepath.Join(root, "relocated")
	if err := os.Rename(parent, renamedParent); err != nil {
		t.Fatal(err)
	}
	const aliasParent = "CONFIG-DIR"
	created, err := f.Mkdir(filepath.Join(aliasParent, "nested"))
	if created {
		t.Error("mkdir reported creation through a protected ancestor alias")
	}
	deny(aliasParent, err)
	if err := os.Rename(renamedParent, parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(renamedSource, source); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(filepath.Join("config-dir", "new-after-restore"), ordinary, false); err != nil {
		t.Fatalf("restored configuration blocked sibling creation: %v", err)
	}
}

func TestProtectedRecursiveDeleteMaintainsSymlinkEntryIdentity(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	writeTestFile(t, source, "protected")
	loaded, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	policy := security.DefaultPolicy()
	policy.FollowSymlinks = true
	guard, err := security.NewPathGuardWithPolicy(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := bindTestConfigProtection(guard, source, loaded)
	if err != nil {
		t.Fatal(err)
	}
	f := mustProtectedFileSystem(t, protection, DefaultLimits())
	swapped := filepath.Join(root, "swapped")
	mkdir(t, swapped)
	link := filepath.Join(swapped, "link")
	target := filepath.Join(root, "target")
	writeTestFile(t, target, "ordinary")
	createTestSymlinkOrSkip(t, target, link)
	rootEffect, err := f.inspectEffect(swapped, true)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := f.inspectTree(rootEffect, true)
	if err != nil || len(entries) != 1 {
		t.Fatalf("inspect link for replacement test: %v", err)
	}
	if err := os.Rename(link, filepath.Join(root, "old-link")); err != nil {
		t.Fatal(err)
	}
	createTestSymlinkOrSkip(t, target, link)
	if err := f.recheckTreeEntry(entries[0]); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("replacement link with the same target was accepted: %v", err)
	}

	ordinary := filepath.Join(root, "ordinary")
	mkdir(t, filepath.Join(ordinary, "z-directory"))
	writeTestFile(t, filepath.Join(ordinary, "z-target"), "ordinary")
	writeTestFile(t, filepath.Join(ordinary, "z-directory", "child"), "ordinary")
	createTestSymlinkOrSkip(t, "z-target", filepath.Join(ordinary, "b-link"))
	createTestSymlinkOrSkip(t, "b-link", filepath.Join(ordinary, "a-chain"))
	createTestSymlinkOrSkip(t, "z-directory", filepath.Join(ordinary, "c-directory-link"))
	if err := f.Delete("ordinary", true); err != nil {
		t.Fatalf("remove ordinary tree with internal links: %v", err)
	}
	if _, err := os.Lstat(ordinary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ordinary tree remains: %v", err)
	}

	blocked := filepath.Join(root, "blocked")
	mkdir(t, blocked)
	sibling := filepath.Join(blocked, "z-sibling")
	writeTestFile(t, sibling, "retain")
	createTestSymlinkOrSkip(t, source, filepath.Join(blocked, "a-link"))
	if err := f.Delete("blocked", true); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("tree with protected target returned %v", err)
	}
	for path, want := range map[string]string{source: "protected", sibling: "retain"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("denied tree delete changed %s: %v", filepath.Base(path), err)
		}
	}
}

func TestConfigurationCopyPreEffectDenialPreservesDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	if err := os.WriteFile(source, []byte("protected"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := bindTestConfigProtection(guard, source, loaded)
	if err != nil {
		t.Fatal(err)
	}
	f := mustProtectedFileSystem(t, protection, DefaultLimits())
	for _, existing := range []bool{false, true} {
		target := filepath.Join(root, fmt.Sprintf("destination-%t", existing))
		if existing {
			if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		operand, err := f.inspectEffect(target, true)
		if err != nil {
			t.Fatal(err)
		}
		checks := 0
		file, err := f.openDestination(operand, 0o600, func() error {
			checks++
			return security.ErrConfigurationAccess
		})
		if file != nil {
			_ = file.Close()
		}
		if !errors.Is(err, security.ErrConfigurationAccess) || file != nil || checks != 1 {
			t.Fatalf("pre-effect denial failed for existing=%t: %v", existing, err)
		}
		data, err := os.ReadFile(target)
		if existing && (err != nil || string(data) != "untouched") || !existing && !os.IsNotExist(err) {
			t.Fatalf("denied copy affected destination for existing=%t", existing)
		}
	}
}

func TestConfigurationRootExchangeDeniesPreparedEffects(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "data")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "config.toml")
	if err := os.WriteFile(source, []byte("protected"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := bindTestConfigProtection(guard, source, loaded)
	if err != nil {
		t.Fatal(err)
	}
	limits := DefaultLimits()
	limits.MaxDeleteEntries = 1
	f := mustProtectedFileSystem(t, protection, limits)
	if created, err := f.Mkdir("ordinary"); err != nil || !created {
		t.Fatalf("ordinary directory was not created: %v", err)
	}
	if created, err := f.Mkdir("ordinary"); err != nil || created {
		t.Fatalf("existing ordinary directory did not remain intact: %v", err)
	}
	if err := f.Write("file", []byte("ordinary"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Mkdir("file"); !errors.Is(err, ErrPathIsNotDirectory) {
		t.Fatalf("ordinary file accepted as a directory: %v", err)
	}
	for _, path := range []string{"ordinary/first", "ordinary/second"} {
		if err := f.Write(path, []byte(path), false); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Delete("ordinary", true); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("recursive delete exceeded the configured protection preflight limit: %v", err)
	}
	entries, err := f.List("ordinary")
	if err != nil || len(entries) != 2 {
		t.Fatal("denied recursive delete changed entries")
	}
	prepared, err := f.inspectEffect(filepath.Join(root, "file"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, filepath.Join(base, "original")); err != nil {
		t.Fatal(err)
	}
	if err := f.recheckEffect(prepared, true); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("prepared effect accepted a removed root: %v", err)
	}
	if _, err := bindTestConfigProtection(guard, source, loaded); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("configuration binding accepted a disappeared root: %v", err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := f.Write("new.txt", []byte("ordinary"), false); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("new root reached authorized effect: %v", err)
	}
	if _, err := f.Read("alias", 128); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("replacement root exposed protected data: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("denied effect created a file in the replacement root")
	}
	if err := os.Rename(source, filepath.Join(base, "original.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := bindTestConfigProtection(guard, source, loaded); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("configuration binding accepted an absent selected pathname: %v", err)
	}
	if _, err := NewProtectedFileSystems([]*security.RootProtection{nil}, limits); !errors.Is(err, security.ErrProtectedPath) {
		t.Fatalf("protected filesystem accepted an unset protection: %v", err)
	}
	limits.MaxDeleteEntries = 0
	if _, err := NewProtectedFileSystems([]*security.RootProtection{protection}, limits); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("protected filesystem accepted invalid limits: %v", err)
	}
}

func TestLocalFileSystemListReturnsFilesAndDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeTestFile(t, filepath.Join(root, "file.txt"), "hello")
	mkdir(t, filepath.Join(root, "subdir"))

	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %#v", len(entries), entries)
	}

	fileEntry := findEntry(t, entries, "file.txt")
	if fileEntry.IsDir {
		t.Fatal("expected file.txt to be a file")
	}

	if fileEntry.Size != int64(len("hello")) {
		t.Fatalf("expected file size %d, got %d", len("hello"), fileEntry.Size)
	}

	dirEntry := findEntry(t, entries, "subdir")
	if !dirEntry.IsDir {
		t.Fatal("expected subdir to be a directory")
	}
}

func TestLocalFileSystemListSortsAcrossBatches(t *testing.T) {
	root := t.TempDir()
	count := listBatchSize*2 + 1
	for index := count - 1; index >= 0; index-- {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("entry-%04d", index)), "x")
	}
	filesystem := mustNewLocalFileSystem(t, root)
	entries, err := filesystem.List(".")
	if err != nil || len(entries) != count {
		t.Fatalf("unexpected batched list count=%d, error=%v", len(entries), err)
	}
	for index, entry := range entries {
		if entry.Name != fmt.Sprintf("entry-%04d", index) || entry.IsDir || entry.Size != 1 {
			t.Fatalf("unexpected entry %d: %#v", index, entry)
		}
	}
}

func TestLocalFileSystemListEmptyDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected empty directory, got %d entries", len(entries))
	}
}

func TestLocalFileSystemListNestedDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nestedDir := filepath.Join(root, "alpha", "beta")

	mkdir(t, nestedDir)
	writeTestFile(t, filepath.Join(nestedDir, "nested.txt"), "nested-content")

	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(filepath.Join("alpha", "beta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := findEntry(t, entries, "nested.txt")
	if entry.IsDir {
		t.Fatal("expected nested.txt to be a file")
	}

	if entry.Size != int64(len("nested-content")) {
		t.Fatalf("expected size %d, got %d", len("nested-content"), entry.Size)
	}
}

func TestLocalFileSystemListRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List("..")

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemListRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List(filepath.Join(root, "file.txt"))

	if !errors.Is(err, security.ErrAbsolutePath) {
		t.Fatalf("expected ErrAbsolutePath, got %v", err)
	}
}

func TestLocalFileSystemListReturnsErrorForMissingDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List("missing")

	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestLocalFileSystemListRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), "secret")
	createTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List("escape-dir")

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestLocalFileSystemListFiltersHiddenEntriesByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "visible.txt"), "visible")
	writeTestFile(t, filepath.Join(root, ".secret"), "secret")

	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 visible entry, got %d: %#v", len(entries), entries)
	}

	if entries[0].Name != "visible.txt" {
		t.Fatalf("expected visible.txt, got %q", entries[0].Name)
	}
}

func TestLocalFileSystemListIncludesHiddenEntriesWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".secret"), "secret")

	filesystem := mustNewLocalFileSystemWithPolicy(t, root, security.Policy{AllowHiddenFiles: true})

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	findEntry(t, entries, ".secret")
}

func TestLocalFileSystemListFiltersSymlinkEntriesByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "target.txt"), "target")
	createTestSymlinkOrSkip(t, filepath.Join(root, "target.txt"), filepath.Join(root, "link.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, entry := range entries {
		if entry.Name == "link.txt" {
			t.Fatalf("expected symlink entry to be filtered, got %#v", entries)
		}
	}
}

func TestLocalFileSystemReadReturnsFileContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "hello world")

	filesystem := mustNewLocalFileSystem(t, root)

	content, err := filesystem.Read("file.txt", 1024)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(content) != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", string(content))
	}
}

func TestReadAtMostRejectsDataBeyondTheLimit(t *testing.T) {
	for _, tc := range []struct {
		data     string
		max      int64
		tooLarge bool
	}{
		{"", 5, false},
		{"exact", 5, false},
		{"larger", 5, true},
	} {
		content, err := readAtMost(strings.NewReader(tc.data), tc.max)
		if tc.tooLarge {
			if !errors.Is(err, ErrFileTooLarge) || content != nil {
				t.Fatalf("over-limit read: content=%q, error=%v", content, err)
			}
		} else if err != nil || string(content) != tc.data {
			t.Fatalf("bounded read: content=%q, error=%v", content, err)
		}
	}
}

func TestLocalFileSystemReadRejectsDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "subdir"))

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Read("subdir", 1024)

	if !errors.Is(err, ErrPathIsDirectory) {
		t.Fatalf("expected ErrPathIsDirectory, got %v", err)
	}
}

func TestLocalFileSystemReadRejectsTooLargeFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "hello world")

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Read("file.txt", 5)

	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestLocalFileSystemReadRejectsZeroLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "hello")

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Read("file.txt", 0)

	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestLocalFileSystemReadRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Read("..", 1024)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemReadRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), "secret")
	createTestSymlinkOrSkip(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "escape.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Read("escape.txt", 1024)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestLocalFileSystemReadRejectsHiddenPathByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".secret"), "secret")

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Read(".secret", 1024)

	if !errors.Is(err, security.ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}
}

func TestLocalFileSystemReadAllowsHiddenPathWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".secret"), "secret")

	filesystem := mustNewLocalFileSystemWithPolicy(t, root, security.Policy{AllowHiddenFiles: true})

	content, err := filesystem.Read(".secret", 1024)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(content) != "secret" {
		t.Fatalf("expected secret content, got %q", string(content))
	}
}

func TestLocalFileSystemStatReturnsMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "hello")

	filesystem := mustNewLocalFileSystem(t, root)

	metadata, err := filesystem.Stat("file.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if metadata.Name != "file.txt" {
		t.Fatalf("expected name %q, got %q", "file.txt", metadata.Name)
	}

	if metadata.IsDir {
		t.Fatal("expected file.txt to be a file")
	}

	if metadata.Size != int64(len("hello")) {
		t.Fatalf("expected size %d, got %d", len("hello"), metadata.Size)
	}
}

func TestLocalFileSystemStatReturnsDirectoryMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "subdir"))

	filesystem := mustNewLocalFileSystem(t, root)

	metadata, err := filesystem.Stat("subdir")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if metadata.Name != "subdir" {
		t.Fatalf("expected name %q, got %q", "subdir", metadata.Name)
	}

	if !metadata.IsDir {
		t.Fatal("expected subdir to be a directory")
	}
}

func TestLocalFileSystemStatRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Stat("..")

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemStatRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), "secret")
	createTestSymlinkOrSkip(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "escape.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Stat("escape.txt")

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestLocalFileSystemStatReturnsNotFoundForMissingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Stat("missing.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLocalFileSystemStatRejectsHiddenMissingPathByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Stat(".missing")

	if !errors.Is(err, security.ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}

}

func TestLocalFileSystemWriteCreatesFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write("created.txt", []byte("created-content"), false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	content := readTestFile(t, filepath.Join(root, "created.txt"))
	if content != "created-content" {
		t.Fatalf("expected %q, got %q", "created-content", content)
	}
}

func TestLocalFileSystemWriteRejectsExistingFileWithoutOverwrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "old-content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write("file.txt", []byte("new-content"), false)

	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got %v", err)
	}

	content := readTestFile(t, filepath.Join(root, "file.txt"))
	if content != "old-content" {
		t.Fatalf("expected file content to remain unchanged, got %q", content)
	}
}

func TestLocalFileSystemWriteOverwritesExistingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "old-content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write("file.txt", []byte("new-content"), true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	content := readTestFile(t, filepath.Join(root, "file.txt"))
	if content != "new-content" {
		t.Fatalf("expected %q, got %q", "new-content", content)
	}
}

func TestLocalFileSystemWriteRejectsDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "subdir"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write("subdir", []byte("content"), true)

	if !errors.Is(err, ErrPathIsDirectory) {
		t.Fatalf("expected ErrPathIsDirectory, got %v", err)
	}
}

func TestLocalFileSystemWriteRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write("..", []byte("content"), false)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemWriteRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write(filepath.Join(root, "file.txt"), []byte("content"), false)

	if !errors.Is(err, security.ErrAbsolutePath) {
		t.Fatalf("expected ErrAbsolutePath, got %v", err)
	}
}

func TestLocalFileSystemWriteRejectsSymlinkedParentEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	createTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write(filepath.Join("escape-dir", "created.txt"), []byte("content"), false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(outside, "created.txt")) {
		t.Fatal("expected outside file to not be created")
	}
}

func TestLocalFileSystemWriteRejectsHiddenTargetByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Write(".secret", []byte("secret"), false)

	if !errors.Is(err, security.ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(root, ".secret")) {
		t.Fatal("expected hidden file to not be created")
	}
}

func TestLocalFileSystemMkdirCreatesDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	created, err := filesystem.Mkdir(filepath.Join("alpha", "beta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}

	info, err := os.Stat(filepath.Join(root, "alpha", "beta"))
	if err != nil {
		t.Fatalf("expected directory to exist, got %v", err)
	}

	if !info.IsDir() {
		t.Fatal("expected created path to be a directory")
	}
}

func TestLocalFileSystemMkdirReportsExistingDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "existing"))
	filesystem := mustNewLocalFileSystem(t, root)

	created, err := filesystem.Mkdir("existing")
	if err != nil || created {
		t.Fatalf("expected created=false without error, got created=%v err=%v", created, err)
	}
}

func TestLocalFileSystemMkdirRejectsExistingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "content")
	filesystem := mustNewLocalFileSystem(t, root)

	created, err := filesystem.Mkdir("file.txt")
	if created || !errors.Is(err, ErrPathIsNotDirectory) {
		t.Fatalf("expected ErrPathIsNotDirectory, got created=%v err=%v", created, err)
	}
}

func TestLocalFileSystemMkdirRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Mkdir("..")

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemMkdirRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Mkdir(filepath.Join(root, "alpha"))

	if !errors.Is(err, security.ErrAbsolutePath) {
		t.Fatalf("expected ErrAbsolutePath, got %v", err)
	}
}

func TestLocalFileSystemMkdirRejectsSymlinkedParentEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	createTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Mkdir(filepath.Join("escape-dir", "created"))

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(outside, "created")) {
		t.Fatal("expected outside directory to not be created")
	}
}

func TestLocalFileSystemMkdirRejectsHiddenDirectoryByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.Mkdir(".secret-dir")

	if !errors.Is(err, security.ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(root, ".secret-dir")) {
		t.Fatal("expected hidden directory to not be created")
	}
}

func TestLocalFileSystemDeleteFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete("file.txt", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "file.txt")) {
		t.Fatal("expected file to be deleted")
	}
}

func TestLocalFileSystemDeleteEmptyDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "empty"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete("empty", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "empty")) {
		t.Fatal("expected directory to be deleted")
	}
}

func TestLocalFileSystemDeleteRejectsNonEmptyDirectoryWithoutRecursive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "dir"))
	writeTestFile(t, filepath.Join(root, "dir", "file.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete("dir", false)

	if !errors.Is(err, ErrDirectoryNotEmpty) {
		t.Fatalf("expected ErrDirectoryNotEmpty, got %v", err)
	}

	if !fileExists(t, filepath.Join(root, "dir", "file.txt")) {
		t.Fatal("expected file to remain")
	}
}

func TestLocalFileSystemDeleteRecursiveDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "dir"))
	writeTestFile(t, filepath.Join(root, "dir", "file.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete("dir", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "dir")) {
		t.Fatal("expected directory to be deleted")
	}
}

func TestLocalFileSystemDeleteRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete("..", false)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemDeleteRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	writeTestFile(t, outsideFile, "secret")
	createTestSymlinkOrSkip(t, outsideFile, filepath.Join(root, "escape.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete("escape.txt", false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if !fileExists(t, outsideFile) {
		t.Fatal("expected outside file to remain")
	}
}

func TestLocalFileSystemDeleteRejectsHiddenPathByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	hiddenPath := filepath.Join(root, ".secret")
	writeTestFile(t, hiddenPath, "secret")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Delete(".secret", false)

	if !errors.Is(err, security.ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}

	if !fileExists(t, hiddenPath) {
		t.Fatal("expected hidden file to remain")
	}
}

func TestLocalFileSystemMoveFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", "target.txt", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "source.txt")) {
		t.Fatal("expected source to be removed")
	}

	content := readTestFile(t, filepath.Join(root, "target.txt"))
	if content != "content" {
		t.Fatalf("expected %q, got %q", "content", content)
	}
}

func TestLocalFileSystemMoveRejectsExistingTargetWithoutOverwrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	writeTestFile(t, filepath.Join(root, "target.txt"), "target")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", "target.txt", false)

	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got %v", err)
	}

	if readTestFile(t, filepath.Join(root, "source.txt")) != "source" {
		t.Fatal("expected source to remain unchanged")
	}

	if readTestFile(t, filepath.Join(root, "target.txt")) != "target" {
		t.Fatal("expected target to remain unchanged")
	}
}

func TestLocalFileSystemMoveOverwritesExistingTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	writeTestFile(t, filepath.Join(root, "target.txt"), "target")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", "target.txt", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "source.txt")) {
		t.Fatal("expected source to be moved")
	}

	if readTestFile(t, filepath.Join(root, "target.txt")) != "source" {
		t.Fatal("expected target to contain source content")
	}
}

func TestLocalFileSystemMoveRejectsSamePathBeforeOverwrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "same.txt")
	writeTestFile(t, path, "content")
	filesystem := mustNewLocalFileSystem(t, root)

	for _, overwrite := range []bool{false, true} {
		err := filesystem.Move("same.txt", "same.txt", overwrite)
		if !errors.Is(err, ErrSamePath) {
			t.Fatalf("overwrite=%v: expected ErrSamePath, got %v", overwrite, err)
		}
		if readTestFile(t, path) != "content" {
			t.Fatalf("overwrite=%v: expected source to remain", overwrite)
		}
	}
}

func TestLocalFileSystemMoveRejectsHardlinkSameFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "alias.txt")
	writeTestFile(t, source, "content")
	if err := os.Link(source, target); err != nil {
		t.Skipf("hardlinks are not available: %v", err)
	}
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", "alias.txt", true)
	if !errors.Is(err, ErrSamePath) {
		t.Fatalf("expected ErrSamePath, got %v", err)
	}
	if readTestFile(t, source) != "content" || readTestFile(t, target) != "content" {
		t.Fatal("expected both hardlink names to remain")
	}
}

func TestLocalFileSystemMoveRenamesDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "old"))
	writeTestFile(t, filepath.Join(root, "old", "file.txt"), "content")
	filesystem := mustNewLocalFileSystem(t, root)

	if err := filesystem.Move("old", "new", false); err != nil {
		t.Fatalf("expected directory rename, got %v", err)
	}
	if readTestFile(t, filepath.Join(root, "new", "file.txt")) != "content" {
		t.Fatal("expected directory content at target")
	}
}

func TestLocalFileSystemMoveMovesDirectoryAcrossParents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "source-parent", "dir"))
	mkdir(t, filepath.Join(root, "target-parent"))
	filesystem := mustNewLocalFileSystem(t, root)

	if err := filesystem.Move(filepath.Join("source-parent", "dir"), filepath.Join("target-parent", "dir"), false); err != nil {
		t.Fatalf("expected same-volume directory move, got %v", err)
	}
	if !fileExists(t, filepath.Join(root, "target-parent", "dir")) {
		t.Fatal("expected target directory")
	}
}

func TestLocalFileSystemMoveMovesFileAcrossParents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "source-parent"))
	mkdir(t, filepath.Join(root, "target-parent"))
	writeTestFile(t, filepath.Join(root, "source-parent", "file.txt"), "content")
	filesystem := mustNewLocalFileSystem(t, root)

	if err := filesystem.Move(filepath.Join("source-parent", "file.txt"), filepath.Join("target-parent", "file.txt"), false); err != nil {
		t.Fatalf("expected same-volume file move, got %v", err)
	}
	if readTestFile(t, filepath.Join(root, "target-parent", "file.txt")) != "content" {
		t.Fatal("expected target file content")
	}
}

func TestLocalFileSystemMoveReturnsNotFoundForMissingSource(t *testing.T) {
	t.Parallel()

	filesystem := mustNewLocalFileSystem(t, t.TempDir())
	if err := filesystem.Move("missing", "target", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLocalFileSystemMoveRejectsExistingTargetTypeCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sourceDir bool
		targetDir bool
	}{
		{"file-to-directory", false, true},
		{"directory-to-file", true, false},
		{"directory-to-directory", true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.sourceDir {
				mkdir(t, filepath.Join(root, "source"))
			} else {
				writeTestFile(t, filepath.Join(root, "source"), "source")
			}
			if test.targetDir {
				mkdir(t, filepath.Join(root, "target"))
			} else {
				writeTestFile(t, filepath.Join(root, "target"), "target")
			}
			filesystem := mustNewLocalFileSystem(t, root)

			err := filesystem.Move("source", "target", true)
			if !errors.Is(err, ErrMoveTypeMismatch) {
				t.Fatalf("expected ErrMoveTypeMismatch, got %v", err)
			}
			if !fileExists(t, filepath.Join(root, "source")) || !fileExists(t, filepath.Join(root, "target")) {
				t.Fatal("expected source and target to remain")
			}
		})
	}
}

func TestLocalFileSystemMoveRejectsDirectoryIntoOwnSubtree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "source", "child"))
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source", filepath.Join("source", "child", "moved"), false)
	if !errors.Is(err, ErrMoveIntoSelf) {
		t.Fatalf("expected ErrMoveIntoSelf, got %v", err)
	}
	if !fileExists(t, filepath.Join(root, "source", "child")) {
		t.Fatal("expected source tree to remain")
	}
}

func TestLocalFileSystemMoveRejectsEffectiveDirectoryIntoOwnSubtree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "source", "child"))
	createTestSymlinkOrSkip(t, filepath.Join(root, "source", "child"), filepath.Join(root, "alias"))
	filesystem := mustNewLocalFileSystemWithPolicy(t, root, security.Policy{FollowSymlinks: true})

	err := filesystem.Move("source", filepath.Join("alias", "moved"), false)
	if !errors.Is(err, ErrMoveIntoSelf) {
		t.Fatalf("expected ErrMoveIntoSelf, got %v", err)
	}
	if !fileExists(t, filepath.Join(root, "source", "child")) {
		t.Fatal("expected source tree to remain")
	}
}

func TestRevalidateMoveStateRejectsReplacedTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "target.txt")
	writeTestFile(t, source, "source")
	writeTestFile(t, target, "target")
	sourceInfo, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	mkdir(t, target)

	err = revalidateMoveState(source, target, sourceInfo, targetInfo, true)
	if !errors.Is(err, ErrMovePathChanged) {
		t.Fatalf("expected ErrMovePathChanged, got %v", err)
	}
	if readTestFile(t, source) != "source" {
		t.Fatal("expected source to remain")
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected replacement directory to remain, info=%#v err=%v", info, err)
	}
}

func TestRenameOnSameVolumeRejectsCrossVolumeWithoutFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "target.txt")
	writeTestFile(t, source, "source")
	renameCalled := false

	err := renameOnSameVolume(source, target, false, func(string, string) error {
		renameCalled = true
		return nil
	})
	if !errors.Is(err, ErrCrossVolumeMoveUnsupported) {
		t.Fatalf("expected ErrCrossVolumeMoveUnsupported, got %v", err)
	}
	if renameCalled {
		t.Fatal("expected no rename or fallback operation")
	}
	if readTestFile(t, source) != "source" {
		t.Fatal("expected source to remain")
	}
	if fileExists(t, target) {
		t.Fatal("expected target to remain absent")
	}
}

func TestLocalFileSystemMoveRejectsTraversalSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("..", "target.txt", false)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemMoveRejectsTraversalTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", "..", false)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemMoveRejectsSymlinkEscapeSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	writeTestFile(t, outsideFile, "secret")
	createTestSymlinkOrSkip(t, outsideFile, filepath.Join(root, "escape.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("escape.txt", "target.txt", false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if !fileExists(t, outsideFile) {
		t.Fatal("expected outside file to remain")
	}
}

func TestLocalFileSystemMoveRejectsSymlinkedTargetParentEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	createTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", filepath.Join("escape-dir", "target.txt"), false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(outside, "target.txt")) {
		t.Fatal("expected outside target to not be created")
	}
}

func TestLocalFileSystemCopyFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("source.txt", "target.txt", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if readTestFile(t, filepath.Join(root, "source.txt")) != "content" {
		t.Fatal("expected source to remain unchanged")
	}

	if readTestFile(t, filepath.Join(root, "target.txt")) != "content" {
		t.Fatal("expected target to contain copied content")
	}
}

func TestLocalFileSystemCopyRejectsDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "source-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("source-dir", "target-dir", false)

	if !errors.Is(err, ErrCopyDirectoryUnsupported) {
		t.Fatalf("expected ErrCopyDirectoryUnsupported, got %v", err)
	}
}

func TestLocalFileSystemCopyRejectsExistingTargetWithoutOverwrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	writeTestFile(t, filepath.Join(root, "target.txt"), "target")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("source.txt", "target.txt", false)

	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got %v", err)
	}

	if readTestFile(t, filepath.Join(root, "target.txt")) != "target" {
		t.Fatal("expected target to remain unchanged")
	}
}

func TestLocalFileSystemCopyOverwritesExistingTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	writeTestFile(t, filepath.Join(root, "target.txt"), "target")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("source.txt", "target.txt", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if readTestFile(t, filepath.Join(root, "source.txt")) != "source" {
		t.Fatal("expected source to remain unchanged")
	}

	if readTestFile(t, filepath.Join(root, "target.txt")) != "source" {
		t.Fatal("expected target to contain source content")
	}
}

func TestLocalFileSystemCopyRejectsTraversalSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("..", "target.txt", false)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemCopyRejectsTraversalTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("source.txt", "..", false)

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemCopyRejectsSymlinkEscapeSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), "secret")
	createTestSymlinkOrSkip(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "escape.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("escape.txt", "target.txt", false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestLocalFileSystemCopyRejectsSymlinkedTargetParentEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	createTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Copy("source.txt", filepath.Join("escape-dir", "target.txt"), false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(outside, "target.txt")) {
		t.Fatal("expected outside target to not be created")
	}
}

func TestLocalFileSystemMoveRenamesFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "old.txt"), "content")

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("old.txt", "new.txt", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "old.txt")) {
		t.Fatal("expected old file to be removed")
	}

	if readTestFile(t, filepath.Join(root, "new.txt")) != "content" {
		t.Fatal("expected renamed file content to match")
	}
}

func TestLocalFileSystemMoveRenameRejectsSymlinkEscapeSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	writeTestFile(t, outsideFile, "secret")
	createTestSymlinkOrSkip(t, outsideFile, filepath.Join(root, "escape.txt"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("escape.txt", "target.txt", false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if !fileExists(t, outsideFile) {
		t.Fatal("expected outside file to remain")
	}
}

func TestLocalFileSystemMoveRenameRejectsSymlinkedTargetParentEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "source")
	createTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	filesystem := mustNewLocalFileSystem(t, root)

	err := filesystem.Move("source.txt", filepath.Join("escape-dir", "target.txt"), false)

	if !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}

	if fileExists(t, filepath.Join(outside, "target.txt")) {
		t.Fatal("expected outside target to not be created")
	}
}

func TestLocalFileSystemListRejectsTooManyEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.txt"), "a")
	writeTestFile(t, filepath.Join(root, "b.txt"), "b")

	filesystem := mustNewLocalFileSystemWithLimits(t, root, Limits{
		MaxWriteBytes:    1024,
		MaxListEntries:   1,
		MaxCopyBytes:     1024,
		MaxDeleteEntries: 10,
	})

	_, err := filesystem.List(".")

	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestLocalFileSystemListCountsFilteredEntriesAgainstScanLimit(t *testing.T) {
	for _, kind := range []string{"hidden", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			target := filepath.Join(root, "visible.txt")
			writeTestFile(t, target, "visible")
			addFiltered := func(index int) {
				name := fmt.Sprintf("entry-%03d", index)
				if kind == "hidden" {
					writeTestFile(t, filepath.Join(root, "."+name), "hidden")
				} else {
					createTestSymlinkOrSkip(t, target, filepath.Join(root, name))
				}
			}

			limits := DefaultLimits()
			limits.MaxListEntries = 2
			filesystem := mustNewLocalFileSystemWithLimits(t, root, limits)
			addFiltered(0)
			entries, err := filesystem.List(".")
			if err != nil || len(entries) != 1 || entries[0].Name != "visible.txt" {
				t.Fatalf("expected one visible entry at the scan limit, got %v, %v", entries, err)
			}

			for index := 1; index <= listBatchSize; index++ {
				addFiltered(index)
			}
			entries, err = filesystem.List(".")
			if !errors.Is(err, ErrLimitExceeded) || entries != nil {
				t.Fatalf("expected scan limit error without a partial listing, got %v, %v", entries, err)
			}
		})
	}
}

func TestLocalFileSystemWriteRejectsContentOverLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystemWithLimits(t, root, Limits{
		MaxWriteBytes:    4,
		MaxListEntries:   10,
		MaxCopyBytes:     1024,
		MaxDeleteEntries: 10,
	})

	err := filesystem.Write("created.txt", []byte("12345"), false)

	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "created.txt")) {
		t.Fatal("expected over-limit write not to create file")
	}
}

func TestLocalFileSystemCopyRejectsSourceOverLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "source.txt"), "12345")
	filesystem := mustNewLocalFileSystemWithLimits(t, root, Limits{
		MaxWriteBytes:    1024,
		MaxListEntries:   10,
		MaxCopyBytes:     4,
		MaxDeleteEntries: 10,
	})

	err := filesystem.Copy("source.txt", "target.txt", false)

	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}

	if fileExists(t, filepath.Join(root, "target.txt")) {
		t.Fatal("expected over-limit copy not to create target")
	}
}

func TestLocalFileSystemDeleteRecursiveRejectsTooManyEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdir(t, filepath.Join(root, "dir"))
	writeTestFile(t, filepath.Join(root, "dir", "a.txt"), "a")
	writeTestFile(t, filepath.Join(root, "dir", "b.txt"), "b")
	filesystem := mustNewLocalFileSystemWithLimits(t, root, Limits{
		MaxWriteBytes:    1024,
		MaxListEntries:   10,
		MaxCopyBytes:     1024,
		MaxDeleteEntries: 1,
	})

	err := filesystem.Delete("dir", true)

	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}

	if !fileExists(t, filepath.Join(root, "dir", "a.txt")) ||
		!fileExists(t, filepath.Join(root, "dir", "b.txt")) {
		t.Fatal("expected over-limit recursive delete not to remove files")
	}
}

func mustNewLocalFileSystem(t *testing.T, root string) *LocalFileSystem {
	t.Helper()

	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	return filesystem
}

func mustNewLocalFileSystemWithPolicy(t *testing.T, root string, policy security.Policy) *LocalFileSystem {
	t.Helper()

	filesystem, err := NewLocalFileSystemWithPolicy(root, policy)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	return filesystem
}

func mustNewLocalFileSystemWithLimits(t *testing.T, root string, limits Limits) *LocalFileSystem {
	t.Helper()

	filesystem, err := NewLocalFileSystemWithPolicyAndLimits(root, security.DefaultPolicy(), limits)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	return filesystem
}

func createTestSymlinkOrSkip(t *testing.T, target string, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" && (errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "required privilege")) {
			t.Skipf("symlink creation is not available in this Windows environment: %v", err)
		}

		t.Fatalf("failed to create symlink: %v", err)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	return string(content)
}

func mkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		return true
	}

	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	t.Fatalf("failed to stat path: %v", err)

	return false
}

func findEntry(t *testing.T, entries []Entry, name string) Entry {
	t.Helper()

	for _, entry := range entries {
		if entry.Name == name {
			return entry
		}
	}

	t.Fatalf("entry %q not found in %#v", name, entries)

	return Entry{}
}
