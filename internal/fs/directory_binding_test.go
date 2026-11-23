package fs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestDirectoryBindingConfigProtectionAndRootDrift(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "root")
	sibling := filepath.Join(root, "sibling")
	if err := os.MkdirAll(sibling, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "active.toml")
	writeTestFile(t, source, "bootstrap source")
	info, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := bindTestConfigProtection(guard, source, info)
	if err != nil {
		t.Fatal(err)
	}
	backend := mustProtectedFileSystem(t, protection, DefaultLimits())
	if _, err := backend.BindDirectory("active.toml"); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("loaded configuration selection returned %v", err)
	}
	if err := os.Rename(source, filepath.Join(root, "loaded.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.BindDirectory("active.toml"); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("selected configuration replaced by a directory returned %v", err)
	}
	binding, err := backend.BindDirectory("sibling")
	if err != nil || binding.Revalidate() != nil {
		t.Fatalf("unrelated sibling did not bind: %v", err)
	}
	if err := os.Rename(root, filepath.Join(filepath.Dir(root), "previous")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := binding.Revalidate(); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("substituted root retained its old binding: %v", err)
	}
	if _, err := backend.BindDirectory("sibling"); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("substituted root admitted a new binding: %v", err)
	}
}

func TestDirectoryBindingNativeLinkRules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	if err := os.Mkdir(allowed, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink("allowed", link); err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, os.ErrPermission) {
			t.Skip("Windows directory symlinks require developer mode or the symlink privilege")
		}
		t.Fatal(err)
	}
	strict, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strict.BindDirectory("linked"); !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("default link policy returned %v", err)
	}
	follow, err := NewLocalFileSystemWithPolicy(root, security.Policy{AllowUNCPaths: true, FollowSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := follow.BindDirectory(`\\server\share`); !errors.Is(err, security.ErrUNCPathDenied) {
		t.Fatalf("UNC operand returned %v", err)
	}
	binding, err := follow.BindDirectory("linked")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(link, filepath.Join(root, "old-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("allowed", link); err != nil {
		t.Fatal(err)
	}
	if err := binding.Revalidate(); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("replaced link retained its old binding: %v", err)
	}
	if err := os.Symlink(allowed, filepath.Join(root, "absolute")); err != nil {
		t.Fatal(err)
	}
	if _, err := follow.BindDirectory("absolute"); !errors.Is(err, security.ErrSymlinkDenied) {
		t.Fatalf("absolute link returned %v", err)
	}
}
