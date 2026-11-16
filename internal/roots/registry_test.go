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

func lookup(t *testing.T, r *Registry, id config.RootID) fs.FileSystem {
	t.Helper()
	f, err := r.Lookup(id)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestRegistryResolvesDistinctRootsAndIsolatesSnapshots(t *testing.T) {
	t.Parallel()
	work, archive := t.TempDir(), t.TempDir()
	r := loadRegistry(t, filepath.Join(t.TempDir(), "config.toml"),
		"[files]\ndefault_id = 'work'\nmax_write_bytes = 4\n[security]\nallow_hidden_files = true\n"+
			rootTable(t, "work", work)+rootTable(t, "archive", archive))
	want := []config.RootID{"archive", "work"}
	if ids := r.IDs(); !slices.Equal(ids, want) || r.DefaultID() != "work" {
		t.Fatalf("unexpected root selection: %v, %v", ids, r.DefaultID())
	}
	for i, id := range want {
		f := lookup(t, r, id)
		if err := f.Write(".data", []byte("large"), false); !errors.Is(err, fs.ErrLimitExceeded) {
			t.Fatalf("root %s did not enforce the configured write limit: %v", id, err)
		}
		if err := f.Write(".data", []byte{byte(i)}, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []config.RootID{"", "Work", "unknown", "work "} {
		if f, err := r.Lookup(id); f != nil || !errors.Is(err, ErrUnknownRoot) {
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
				f, err := r.Lookup(id)
				if err != nil {
					t.Error(err)
					return
				}
				got, err := f.Read(".data", 1)
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
	document := "[files]\ndefault_id = 'outer'\n[security]\nfollow_symlinks = true\n" +
		rootTable(t, "outer", outer) + rootTable(t, "inner", inner)
	r := loadRegistry(t, source, document)
	outerFS, innerFS := lookup(t, r, "outer"), lookup(t, r, "inner")
	loadedAlias := filepath.Join(inner, "loaded.toml")
	if err := os.Link(source, loadedAlias); err != nil {
		t.Fatal(err)
	}
	if err := innerFS.Write("sibling.txt", []byte("sibling"), false); err != nil {
		t.Fatal(err)
	}
	expectDenied := func(err error) {
		t.Helper()
		if !errors.Is(err, security.ErrConfigurationAccess) {
			t.Errorf("expected configuration protection, got %v", err)
		}
	}
	exercise := func(f fs.FileSystem, protected, sibling string) {
		t.Helper()
		data, err := f.Read(protected, config.MaxDocumentBytes)
		expectDenied(err)
		if len(data) != 0 {
			t.Error("protected read returned bytes")
		}
		expectDenied(f.Copy(protected, sibling, true))
		expectDenied(f.Copy(sibling, protected, true))
		expectDenied(f.Write(protected, []byte("replacement"), true))
		expectDenied(f.Move(protected, sibling, true))
		expectDenied(f.Move(sibling, protected, true))
		expectDenied(f.Delete(protected, false))
		if info, err := f.Stat(protected); err != nil || info.IsDir {
			t.Errorf("protected stat lost allowed metadata: %v", err)
		}
		if data, err := f.Read(sibling, 20); err != nil || string(data) != "sibling" {
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
		exercise(outerFS, filepath.Join("settings", alias), filepath.Join("settings", "sibling.txt"))
		exercise(innerFS, alias, "sibling.txt")
	}
	expectDenied(outerFS.Move("settings", "relocated", false))
	expectDenied(outerFS.Delete("settings", true))
	if entries, err := innerFS.List("."); err != nil || len(entries) < 3 {
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
	exercise(outerFS, filepath.Join("settings", "current.toml"), filepath.Join("settings", "sibling.txt"))
	exercise(innerFS, "loaded.toml", "sibling.txt")
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
		"[files]\ndefault_id = 'work'\n"+rootTable(t, "work", work)+rootTable(t, "archive", archive))
	if err := os.Rename(work, filepath.Join(base, "previous")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, filepath.Join(work, "file"), []byte("replacement root"))
	f := lookup(t, r, "work")
	_, readErr := f.Read("file", 64)
	_, statErr := f.Stat("file")
	_, listErr := f.List(".")
	for _, err := range []error{readErr, statErr, listErr, f.Write("new", []byte("rejected"), false)} {
		if !errors.Is(err, security.ErrConfigurationAccess) {
			t.Errorf("root drift reached an operation: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "new")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("root drift reached a write")
	}
	if err := lookup(t, r, "archive").Write("other.txt", []byte("allowed"), false); err != nil {
		t.Fatal(err)
	}
}
