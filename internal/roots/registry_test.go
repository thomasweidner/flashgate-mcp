package roots

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func rootTable(t *testing.T, id config.RootID, path string) string {
	t.Helper()
	value, err := json.Marshal(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("[files.roots.%s]\npath = %s\n", id, value)
}

func loadRegistry(t *testing.T, source, document string) *Registry {
	t.Helper()
	testconfig.Write(t, source, []byte(document))
	cfg, err := config.Load(config.Source{Role: config.Direct, Path: source})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestRegistryResolvesDistinctRootsAndIsolatesSnapshots(t *testing.T) {
	t.Parallel()
	work, archive := t.TempDir(), t.TempDir()
	r := loadRegistry(t, filepath.Join(t.TempDir(), "config.toml"),
		"[files]\ndefault_id = 'work'\nmax_write_size = 4\n[security]\nallow_hidden_files = true\n"+
			rootTable(t, "work", work)+"access = ['read', 'write']\n"+
			rootTable(t, "archive", archive)+"access = ['read', 'write']\n")
	want := []config.RootID{"archive", "work"}
	if ids := r.IDs(); !slices.Equal(ids, want) {
		t.Fatalf("unexpected root selection: %v", ids)
	}
	for i, id := range want {
		operand := fs.Operand{RootID: id, Path: ".data"}
		if err := r.Write(operand, []byte("large"), false); !errors.Is(err, fs.ErrLimitExceeded) {
			t.Fatalf("root %s did not enforce the configured write limit: %v", id, err)
		}
		if err := r.Write(operand, []byte{byte(i)}, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []config.RootID{"", "Work", "unknown", "work "} {
		if _, err := r.Read(fs.Operand{RootID: id, Path: ".data"}, 1); !errors.Is(err, fs.ErrUnknownRoot) {
			t.Errorf("invalid ID %q resolved: %v", id, err)
		}
	}
	var concurrent sync.WaitGroup
	for range 8 {
		concurrent.Go(func() {
			ids := r.IDs()
			if !slices.Equal(ids, want) {
				t.Errorf("root enumeration changed: %v", ids)
				return
			}
			ids[0] = "overwritten"
			for i, id := range want {
				got, err := r.Read(fs.Operand{RootID: id, Path: ".data"}, 1)
				if err != nil || len(got) != 1 || got[0] != byte(i) {
					t.Errorf("root %s returned the wrong payload: %v", id, err)
				}
			}
		})
	}
	concurrent.Wait()
}

func TestRegistryProtectsConfigThroughOverlappingRoots(t *testing.T) {
	t.Parallel()
	outer := t.TempDir()
	inner := filepath.Join(outer, "settings")
	if err := os.Mkdir(inner, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(inner, "config.toml")
	document := "[files]\ndefault_id = 'outer'\n" +
		rootTable(t, "outer", outer) + "policy = 'workspace'\n" +
		rootTable(t, "inner", inner) + "policy = 'workspace'\n" +
		"[policy.workspace]\naccess = ['read', 'write']\nfollow_symlinks = true\n"
	r := loadRegistry(t, source, document)
	outerPath := func(path string) fs.Operand { return fs.Operand{RootID: "outer", Path: path} }
	innerPath := func(path string) fs.Operand { return fs.Operand{RootID: "inner", Path: path} }
	loadedAlias := filepath.Join(inner, "loaded.toml")
	if err := os.Link(source, loadedAlias); err != nil {
		t.Fatal(err)
	}
	if err := r.Write(innerPath("sibling.txt"), []byte("sibling"), false); err != nil {
		t.Fatal(err)
	}
	expectDenied := func(err error) {
		t.Helper()
		if !errors.Is(err, security.ErrConfigurationAccess) {
			t.Errorf("expected configuration protection, got %v", err)
		}
	}
	exercise := func(protected, sibling fs.Operand) {
		t.Helper()
		data, err := r.Read(protected, config.MaxDocumentBytes)
		expectDenied(err)
		if len(data) != 0 {
			t.Error("protected read returned bytes")
		}
		expectDenied(r.Copy(protected, sibling, true))
		expectDenied(r.Copy(sibling, protected, true))
		expectDenied(r.Write(protected, []byte("replacement"), true))
		expectDenied(r.Move(protected, sibling, true))
		expectDenied(r.Move(sibling, protected, true))
		expectDenied(r.Delete(protected, false))
		if info, err := r.Stat(protected); err != nil || info.IsDir {
			t.Errorf("protected stat lost allowed metadata: %v", err)
		}
		if data, err := r.Read(sibling, 20); err != nil || string(data) != "sibling" {
			t.Errorf("denied operation changed the sibling target: %v", err)
		}
	}
	aliases := []string{"config.toml", "loaded.toml"}
	if runtime.GOOS == "windows" {
		aliases = append(aliases, "CONFIG.TOML")
	}
	if err := os.Symlink("config.toml", filepath.Join(inner, "symbolic.toml")); err == nil {
		aliases = append(aliases, "symbolic.toml")
	} else if runtime.GOOS != "windows" {
		t.Fatal(err)
	}
	for _, alias := range aliases {
		exercise(outerPath(filepath.Join("settings", alias)), outerPath("settings/sibling.txt"))
		exercise(innerPath(alias), innerPath("sibling.txt"))
	}
	expectDenied(r.Move(outerPath("settings"), outerPath("relocated"), false))
	expectDenied(r.Delete(outerPath("settings"), true))
	if entries, err := r.List(innerPath(".")); err != nil || len(entries) < 3 {
		t.Fatalf("protected directory lost allowed listing metadata: %v", err)
	}

	if err := os.Rename(source, filepath.Join(inner, "original.toml")); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, source, []byte("replacement"))
	currentAlias := filepath.Join(inner, "current.toml")
	if err := os.Link(source, currentAlias); err != nil {
		t.Fatal(err)
	}
	exercise(outerPath(filepath.Join("settings", "current.toml")), outerPath("settings/sibling.txt"))
	exercise(innerPath("loaded.toml"), innerPath("sibling.txt"))
	for path, want := range map[string]string{source: "replacement", loadedAlias: document} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Errorf("protected object changed: %v", err)
		}
	}
}

func TestRegistryDeniesDriftOnOneRootAndKeepsSiblingRootUsable(t *testing.T) {
	t.Parallel()
	base, archive := t.TempDir(), t.TempDir()
	work := filepath.Join(base, "work")
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	r := loadRegistry(t, filepath.Join(base, "config.toml"),
		"[files]\ndefault_id = 'work'\n"+rootTable(t, "work", work)+"access = ['read', 'write']\n"+
			rootTable(t, "archive", archive)+"access = ['write']\n")
	if err := os.Rename(work, filepath.Join(base, "previous")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, filepath.Join(work, "file"), []byte("replacement root"))
	workPath := func(path string) fs.Operand { return fs.Operand{RootID: "work", Path: path} }
	_, readErr := r.Read(workPath("file"), 64)
	_, statErr := r.Stat(workPath("file"))
	_, listErr := r.List(workPath("."))
	for _, err := range []error{readErr, statErr, listErr, r.Write(workPath("new"), []byte("rejected"), false)} {
		if !errors.Is(err, security.ErrConfigurationAccess) {
			t.Errorf("root drift reached an operation: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "new")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("root drift reached a write")
	}
	if err := r.Write(fs.Operand{RootID: "archive", Path: "other.txt"}, []byte("allowed"), false); err != nil {
		t.Fatal(err)
	}
}
