package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func protectedTestExclusions(t *testing.T, dataRoot, patterns string, follow bool) *LocalFileSystem {
	t.Helper()
	source := filepath.Join(t.TempDir(), "active.toml")
	if err := os.WriteFile(source, []byte("protected"), 0o600); err != nil {
		t.Fatal(err)
	}
	document := "[files]\ndefault_id='work'\n[files.work]\npath=" + strconv.Quote(filepath.ToSlash(dataRoot)) +
		"\nexcluded=" + patterns + "\n"
	cfg, err := config.Decode([]byte(document), config.Direct)
	if err != nil {
		t.Fatal(err)
	}
	info, err := security.PathIdentity(source)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := security.NewPathGuardWithPolicy(dataRoot, security.Policy{FollowSymlinks: follow})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := bindTestConfigProtection(guard, source, info)
	if err != nil {
		t.Fatal(err)
	}
	backends, err := NewProtectedRootFileSystemsWithExclusions([]*security.RootProtection{binding},
		[]Limits{DefaultLimits()}, []config.PathExclusions{cfg.Files().Roots()[0].Excluded})
	if err != nil {
		t.Fatal(err)
	}
	return backends[0]
}

func TestExclusionProjectedMoveRechecksBeforeRename(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	f := protectedTestExclusions(t, root, "['dest/late']", false)
	f.beforeNative = func(stage string) {
		if stage == "rename" {
			if err := os.Mkdir(filepath.Join(source, "late"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.Move("source", "dest", false); !errors.Is(err, ErrPathExcluded) {
		t.Fatalf("projected descendant introduced at rename boundary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "late")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dest")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blocked rename reached the destination: %v", err)
	}
}

func TestExclusionRechecksLinkDriftBeforeTruncate(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"excluded", "ordinary"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link, replacement := filepath.Join(root, "selected"), filepath.Join(root, "replacement")
	if err := os.Symlink("ordinary", link); err != nil {
		t.Skipf("relative symlink fixture unavailable: %v", err)
	}
	if err := os.Symlink("excluded", replacement); err != nil {
		t.Fatal(err)
	}
	f := protectedTestExclusions(t, root, "['excluded']", true)
	f.beforeNative = func(stage string) {
		if stage == "truncate" {
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(replacement, link); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.Write("selected", []byte("changed"), true); !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("expected drift to stop truncation: %v", err)
	}
	for _, name := range []string{"excluded", "ordinary"} {
		if b, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(b) != name {
			t.Fatalf("drift reached an effect: %v", err)
		}
	}
}
