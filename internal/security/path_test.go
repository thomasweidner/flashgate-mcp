package security

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewPathGuardRejectsEmptyRoot(t *testing.T) {
	t.Parallel()

	_, err := NewPathGuard("")

	if !errors.Is(err, ErrEmptyRoot) {
		t.Fatalf("expected ErrEmptyRoot, got %v", err)
	}
}

func TestNewPathGuardNormalizesRootToAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	guard, err := NewPathGuard(root)
	if err != nil {
		t.Fatalf("expected no error for root %q, got %v", root, err)
	}

	if !filepath.IsAbs(guard.Root()) {
		t.Fatalf("expected absolute root path, got %q", guard.Root())
	}

	expected, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if guard.Root() != expected {
		t.Fatalf("expected root %q, got %q", expected, guard.Root())
	}
}

func TestNewPathGuardRejectsNonExistentRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "missing")

	_, err := NewPathGuard(root)

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestNewPathGuardRejectsFileRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root.txt")
	writeSecurityTestFile(t, root, "not a directory")

	_, err := NewPathGuard(root)

	if !errors.Is(err, ErrRootNotDirectory) {
		t.Fatalf("expected ErrRootNotDirectory, got %v", err)
	}
}

func TestNewPathGuardAcceptsDirectoryWithTrailingSeparator(t *testing.T) {
	t.Parallel()

	root := t.TempDir() + string(filepath.Separator)
	guard, err := NewPathGuard(root)
	if err != nil {
		t.Fatalf("expected trailing separator root to be accepted, got %v", err)
	}
	if !filepath.IsAbs(guard.Root()) {
		t.Fatalf("expected absolute root, got %q", guard.Root())
	}
}

func TestNewPathGuardAcceptsFilesystemRoot(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	volumeRoot := filepath.VolumeName(tempRoot) + string(filepath.Separator)
	if filepath.VolumeName(tempRoot) == "" {
		volumeRoot = string(filepath.Separator)
	}

	if _, err := NewPathGuardWithPolicy(volumeRoot, Policy{AllowHiddenFiles: true}); err != nil {
		t.Fatalf("expected filesystem root to be accepted, got %v", err)
	}
}

func TestNewPathGuardSymlinkRootPolicy(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "root-link")
	createSecurityTestSymlinkOrSkip(t, target, link)

	if _, err := NewPathGuard(link); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatalf("expected symlink root to be denied by default, got %v", err)
	}
	if _, err := NewPathGuardWithPolicy(link, Policy{FollowSymlinks: true}); err != nil {
		t.Fatalf("expected followed directory symlink root to be accepted, got %v", err)
	}
}

func TestNewPathGuardRejectsSymlinkRootToFileWhenFollowing(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "target.txt")
	writeSecurityTestFile(t, target, "file")
	link := filepath.Join(t.TempDir(), "root-link")
	createSecurityTestSymlinkOrSkip(t, target, link)

	_, err := NewPathGuardWithPolicy(link, Policy{FollowSymlinks: true})
	if !errors.Is(err, ErrRootNotDirectory) {
		t.Fatalf("expected ErrRootNotDirectory, got %v", err)
	}
}

func TestResolveRejectsEmptyAndBlankUserPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	for _, path := range []string{"", " \t "} {
		if _, err := guard.Resolve(path); !errors.Is(err, ErrEmptyPath) {
			t.Errorf("expected empty-path denial for %q, got %v", path, err)
		}
	}
}

func TestResolveExistingAcceptsExistingRelativePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSecurityTestFile(t, filepath.Join(root, "alpha.txt"), "alpha")
	guard := mustNewPathGuard(t, root)

	safePath, err := guard.ResolveExisting("alpha.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join(guard.Root(), "alpha.txt")
	if safePath.String() != expected {
		t.Fatalf("expected %q, got %q", expected, safePath.String())
	}
}

func TestResolveRejectsHiddenDotPathByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdirSecurityTestDir(t, filepath.Join(root, ".git"))
	writeSecurityTestFile(t, filepath.Join(root, ".git", "config"), "config")
	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveExisting(filepath.Join(".git", "config"))

	if !errors.Is(err, ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}
}

func TestResolveAllowsHiddenDotPathWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdirSecurityTestDir(t, filepath.Join(root, ".git"))
	writeSecurityTestFile(t, filepath.Join(root, ".git", "config"), "config")
	guard := mustNewPathGuardWithPolicy(t, root, Policy{AllowHiddenFiles: true})

	_, err := guard.ResolveExisting(filepath.Join(".git", "config"))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestResolveForCreateRejectsHiddenTargetByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveForCreate(".secret")

	if !errors.Is(err, ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}
}

func TestResolveForCreateAllowsNormalExistingParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdirSecurityTestDir(t, filepath.Join(root, "alpha"))
	guard := mustNewPathGuard(t, root)

	safePath, err := guard.ResolveForCreate(filepath.Join("alpha", "created.txt"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join(guard.Root(), "alpha", "created.txt")
	if safePath.String() != expected {
		t.Fatalf("expected %q, got %q", expected, safePath.String())
	}
}

func TestResolveExistingRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeSecurityTestFile(t, filepath.Join(outside, "secret.txt"), "secret")
	createSecurityTestSymlinkOrSkip(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "escape.txt"))

	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveExisting("escape.txt")

	if !errors.Is(err, ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestResolveExistingRejectsSymlinkByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSecurityTestFile(t, filepath.Join(root, "target.txt"), "target")
	createSecurityTestSymlinkOrSkip(t, filepath.Join(root, "target.txt"), filepath.Join(root, "link.txt"))

	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveExisting("link.txt")

	if !errors.Is(err, ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestResolveExistingAllowsInRootSymlinkWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSecurityTestFile(t, filepath.Join(root, "target.txt"), "target")
	createSecurityTestSymlinkOrSkip(t, filepath.Join(root, "target.txt"), filepath.Join(root, "link.txt"))

	guard := mustNewPathGuardWithPolicy(t, root, Policy{FollowSymlinks: true})

	_, err := guard.ResolveExisting("link.txt")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestResolveExistingRejectsSymlinkEscapeEvenWhenFollowingSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	writeSecurityTestFile(t, filepath.Join(outside, "secret.txt"), "secret")
	createSecurityTestSymlinkOrSkip(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "escape.txt"))

	guard := mustNewPathGuardWithPolicy(t, root, Policy{FollowSymlinks: true})

	_, err := guard.ResolveExisting("escape.txt")

	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
}

func TestResolveForCreateRejectsSymlinkedParentEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	createSecurityTestSymlinkOrSkip(t, outside, filepath.Join(root, "escape-dir"))

	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveForCreate(filepath.Join("escape-dir", "created.txt"))

	if !errors.Is(err, ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestResolveForCreateRejectsSymlinkedParentByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdirSecurityTestDir(t, filepath.Join(root, "real-dir"))
	createSecurityTestSymlinkOrSkip(t, filepath.Join(root, "real-dir"), filepath.Join(root, "link-dir"))

	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveForCreate(filepath.Join("link-dir", "created.txt"))

	if !errors.Is(err, ErrSymlinkDenied) {
		t.Fatalf("expected ErrSymlinkDenied, got %v", err)
	}
}

func TestResolveForCreateAllowsSymlinkedParentWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkdirSecurityTestDir(t, filepath.Join(root, "real-dir"))
	createSecurityTestSymlinkOrSkip(t, filepath.Join(root, "real-dir"), filepath.Join(root, "link-dir"))

	guard := mustNewPathGuardWithPolicy(t, root, Policy{FollowSymlinks: true})

	_, err := guard.ResolveForCreate(filepath.Join("link-dir", "created.txt"))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestResolveAcceptsRelativePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	safePath, err := guard.Resolve(filepath.Join("alpha", "beta.txt"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join(guard.Root(), "alpha", "beta.txt")

	if safePath.String() != expected {
		t.Fatalf("expected %q, got %q", expected, safePath.String())
	}
}

func TestResolveRejectsParentTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	_, err := guard.Resolve("..")

	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestResolveRejectsNestedParentTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	_, err := guard.Resolve(filepath.Join("..", "outside.txt"))

	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestResolveRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	absolutePath := filepath.Join(root, "file.txt")
	_, err := guard.Resolve(absolutePath)

	if !errors.Is(err, ErrAbsolutePath) {
		t.Fatalf("expected ErrAbsolutePath, got %v", err)
	}
}

func TestSafePathBaseAndDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	safePath, err := guard.Resolve(filepath.Join("dir", "file.txt"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if safePath.Base() != "file.txt" {
		t.Fatalf("expected base %q, got %q", "file.txt", safePath.Base())
	}

	expectedDir := filepath.Join(guard.Root(), "dir")
	if safePath.Dir() != expectedDir {
		t.Fatalf("expected dir %q, got %q", expectedDir, safePath.Dir())
	}
}

func TestResolveRejectsWindowsAbsolutePathOnWindows(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("windows-specific test")
	}

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	_, err := guard.Resolve(`C:\Windows\System32`)

	if !errors.Is(err, ErrAbsolutePath) {
		t.Fatalf("expected ErrAbsolutePath, got %v", err)
	}
}

func TestNewPathGuardRejectsUNCRootByDefaultOnWindows(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("windows-specific test")
	}

	_, err := NewPathGuard(`\\server\share`)

	if !errors.Is(err, ErrUNCPathDenied) {
		t.Fatalf("expected ErrUNCPathDenied, got %v", err)
	}
}

func TestResolveRejectsUNCUserPathByDefaultOnWindows(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("windows-specific test")
	}

	root := t.TempDir()
	guard := mustNewPathGuard(t, root)

	_, err := guard.Resolve(`\\server\share\file.txt`)

	if !errors.Is(err, ErrUNCPathDenied) {
		t.Fatalf("expected ErrUNCPathDenied, got %v", err)
	}
}

func mustNewPathGuard(t *testing.T, root string) *PathGuard {
	t.Helper()

	guard, err := NewPathGuard(root)
	if err != nil {
		t.Fatalf("expected no error for root %q, got %v", root, err)
	}

	return guard
}

func mustNewPathGuardWithPolicy(t *testing.T, root string, policy Policy) *PathGuard {
	t.Helper()

	guard, err := NewPathGuardWithPolicy(root, policy)
	if err != nil {
		t.Fatalf("expected no error for root %q, got %v", root, err)
	}

	return guard
}

func writeSecurityTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func mkdirSecurityTestDir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
}

func createSecurityTestSymlinkOrSkip(t *testing.T, target string, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" && (errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "required privilege")) {
			t.Skipf("symlink creation is not available in this Windows environment: %v", err)
		}

		t.Fatalf("failed to create symlink: %v", err)
	}
}
