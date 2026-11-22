package roots

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestPathExclusionEffectsAndAliases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, dir := range []string{"A/X", "A/X2", "source/nested", "clean/keep"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sandbox := filepath.Join(root, "A/X")
	document := "[files]\ndefault_id='outer'\n" +
		rootTable(t, "outer", root) + "policy='base'\n" +
		rootTable(t, "aliased", root) + "policy='base'\nexcluded=[]\n" +
		rootTable(t, "nested", sandbox) + "policy='base'\nexcluded=[]\n" +
		"[policy.base]\naccess=['files.read','files.write']\n" +
		"excluded=['A/X','source/nested/hid?.txt','moved/nested','missing/blocked','logs/my*.log']\n"
	source := filepath.Join(t.TempDir(), "active.toml")
	r := loadRegistry(t, source, document)
	for _, path := range []string{"A/X/value.txt", "A/X2/visible.txt", "source/nested/hide.txt", "source/nested/keep.txt", "clean/keep/file.txt"} {
		testconfig.Write(t, filepath.Join(root, path), []byte("original"))
	}
	outer := func(path string) fs.Operand { return fs.Operand{RootID: "outer", Path: path} }
	protected := outer("A/X/value.txt")
	sibling := outer("A/X2/visible.txt")
	denied := func(err error) {
		t.Helper()
		if !errors.Is(err, fs.ErrPathExcluded) {
			t.Errorf("expected redacted exclusion denial, got %v", err)
		}
	}
	_, err := r.List(outer("A/X"))
	denied(err)
	_, err = r.List(fs.Operand{RootID: "nested", Path: "."})
	denied(err)
	_, err = r.Read(fs.Operand{RootID: "aliased", Path: protected.Path}, 40)
	denied(err)
	_, err = r.Read(protected, 40)
	denied(err)
	_, err = r.Stat(outer("A/X/nonexistent/deeper.txt"))
	denied(err)
	_, err = r.Read(outer("A/X/nonexistent.txt"), 40)
	denied(err)
	denied(r.Write(protected, []byte("replacement"), true))
	denied(r.Write(outer("A/X/not-created.txt"), []byte("replacement"), false))
	_, err = r.Mkdir(outer("missing/blocked/leaf"))
	denied(err)
	_, err = r.Mkdir(outer("A/X/not-created"))
	denied(err)
	denied(r.Delete(outer("source"), true))
	denied(r.Delete(protected, false))
	denied(r.Move(protected, sibling, true))
	denied(r.Move(sibling, protected, true))
	denied(r.Move(outer("source"), outer("renamed"), false))
	if err := os.Rename(filepath.Join(root, "source/nested/hide.txt"), filepath.Join(root, "source/nested/shown.txt")); err != nil {
		t.Fatal(err)
	}
	denied(r.Move(outer("source"), outer("moved"), false))
	denied(r.Copy(protected, sibling, true))
	denied(r.Copy(sibling, protected, true))
	for _, path := range []string{"missing", "moved", "renamed", "A/X/not-created", "A/X/not-created.txt"} {
		if _, err := os.Lstat(filepath.Join(root, path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("denied operation created %s: %v", path, err)
		}
	}
	for _, path := range []string{"A/X/value.txt", "A/X2/visible.txt", "source/nested/keep.txt", "source/nested/shown.txt"} {
		if data, err := os.ReadFile(filepath.Join(root, path)); err != nil || string(data) != "original" {
			t.Errorf("denied operation changed %s: %v", path, err)
		}
	}
	listed, err := r.List(outer("A"))
	if err != nil || len(listed) != 1 || listed[0].Name != "X2" {
		t.Fatalf("listing did not omit excluded directory: %v, %v", listed, err)
	}
	if err := r.Move(outer("clean"), outer("allowed"), false); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(outer("allowed"), true); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(root, protected.Path), filepath.Join(root, "ordinary-alias.txt")); err != nil {
		t.Fatal(err)
	}
	alias := outer("ordinary-alias.txt")
	if data, err := r.Read(alias, 40); err != nil || string(data) != "original" {
		t.Fatalf("configuration confused path and ordinary file identity: %v", err)
	}
	if err := r.Write(alias, []byte("shared"), true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, protected.Path)); err != nil || string(data) != "shared" {
		t.Fatalf("allowed hardlink did not retain ordinary shared content semantics: %v", err)
	}
	if err := os.Link(source, filepath.Join(root, "configuration-alias.toml")); err != nil {
		t.Fatal(err)
	}
	_, err = r.Read(outer("configuration-alias.toml"), 40)
	if !errors.Is(err, security.ErrConfigurationAccess) {
		t.Fatalf("active configuration alias lost identity protection: %v", err)
	}
}

func TestNativeCaseAndExclusionScanBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"secret.log", "sibling"} {
		testconfig.Write(t, filepath.Join(root, name), []byte("content"))
	}
	r := loadRegistry(t, filepath.Join(t.TempDir(), "active.toml"), "[files]\ndefault_id='work'\nmax_list_entries=1\n"+
		rootTable(t, "work", root)+"excluded=['SECRET.LOG']\naccess=['files.read','files.write']\n")
	_, err := r.Read(fs.Operand{RootID: "work", Path: "secret.log"}, 40)
	if runtime.GOOS == "windows" && !errors.Is(err, fs.ErrPathExcluded) || runtime.GOOS == "linux" && err != nil {
		t.Errorf("unexpected native case result: %v", err)
	}
	_, err = r.Read(fs.Operand{RootID: "work", Path: "SECRET.LOG"}, 40)
	if !errors.Is(err, fs.ErrPathExcluded) {
		t.Errorf("exact excluded spelling was not blocked: %v", err)
	}
	if entries, err := r.List(fs.Operand{RootID: "work", Path: "."}); len(entries) != 0 || !errors.Is(err, fs.ErrLimitExceeded) {
		t.Fatalf("listing budget failed to count omitted names: %v, %v", entries, err)
	}
}

func TestPathExclusionLinkEntriesAndTargets(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, path := range []string{"private", "otherwise"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o700); err != nil {
			t.Fatal(err)
		}
		testconfig.Write(t, filepath.Join(root, path, "data.txt"), []byte("fixture"))
	}
	for _, pair := range [][2]string{
		{"private", "private-alias"}, {"private/data.txt", "indirect"},
		{"otherwise/data.txt", "excluded-link"}, {"excluded-link", "chain"},
		{"private/nonexistent", "dangling"}, {"otherwise", "virtual"},
	} {
		if err := os.Symlink(pair[0], filepath.Join(root, pair[1])); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("relative symlink fixture unavailable: %v", err)
			}
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join("root", "private"), filepath.Join(base, "bound-alias")); err != nil {
		t.Fatal(err)
	}
	r := loadRegistry(t, filepath.Join(t.TempDir(), "active.toml"), "[files]\ndefault_id='work'\n"+
		rootTable(t, "work", root)+"policy='workspace'\nexcluded=['private','excluded-link','virtual/data.txt']\n"+
		rootTable(t, "alias", filepath.Join(base, "bound-alias"))+"policy='workspace'\n"+
		"[policy.workspace]\nfollow_symlinks=true\naccess=['files.read','files.write']\n")
	for _, path := range []string{"private-alias/data.txt", "private-alias/missing.txt", "indirect", "excluded-link", "chain", "dangling", "virtual/data.txt"} {
		_, err := r.Read(fs.Operand{RootID: "work", Path: path}, 40)
		if !errors.Is(err, fs.ErrPathExcluded) {
			t.Errorf("%s bypassed exclusion: %v", path, err)
		}
	}
	if _, err := r.Read(fs.Operand{RootID: "alias", Path: "data.txt"}, 40); !errors.Is(err, fs.ErrPathExcluded) {
		t.Errorf("aliased root bypassed exclusion: %v", err)
	}
	entries, err := r.List(fs.Operand{RootID: "work", Path: "."})
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	slices.Sort(names)
	if err != nil || !slices.Equal(names, []string{"otherwise", "virtual"}) {
		t.Errorf("symlink exclusion listing: %v, %v", names, err)
	}
	if data, err := r.Read(fs.Operand{RootID: "work", Path: "otherwise/data.txt"}, 40); err != nil || string(data) != "fixture" {
		t.Fatalf("ordinary sibling lost access: %v", err)
	}
}
