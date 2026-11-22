package roots

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestDeniedOperationsDoNotDispatch(t *testing.T) {
	t.Parallel()
	// Any backend call would panic. The registry must deny before dispatch.
	catalog, err := capability.NewCatalog(capability.BaseDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	newSet := func(c capability.Catalog, ids ...capability.ID) capability.Set {
		t.Helper()
		set, err := c.NewSet(ids...)
		if err != nil {
			t.Fatal(err)
		}
		return set
	}
	profile, err := capability.NewPolicy(newSet(catalog, capability.FilesRead, capability.FilesWrite), capability.RiskMutation)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := fs.BindOperations(catalog)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := capability.NewCatalog(capability.BaseDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	r := Registry{profile: profile, operations: operations, entries: map[config.RootID]entry{
		"readonly":  {access: newSet(catalog, capability.FilesRead)},
		"writeonly": {access: newSet(catalog, capability.FilesWrite)},
		"denied":    {access: newSet(catalog)},
		"foreign":   {access: newSet(foreign, capability.FilesRead)},
	}}
	read := fs.Operand{RootID: "readonly", Path: "file"}
	write := fs.Operand{RootID: "writeonly", Path: "file"}
	denied := fs.Operand{RootID: "denied", Path: "file"}
	target := fs.Operand{RootID: "readonly", Path: "target"}
	writeTarget := fs.Operand{RootID: "writeonly", Path: "target"}
	_, listErr := r.List(write)
	_, readErr := r.Read(write, 10)
	_, statErr := r.Stat(write)
	_, foreignErr := r.Read(fs.Operand{RootID: "foreign", Path: "file"}, 10)
	_, mkdirErr := r.Mkdir(read)
	for name, err := range map[string]error{
		"list write-only":         listErr,
		"read write-only":         readErr,
		"stat write-only":         statErr,
		"write read-only":         r.Write(read, []byte("new"), true),
		"mkdir read-only":         mkdirErr,
		"delete read-only":        r.Delete(read, true),
		"copy lacks target write": r.Copy(read, target, true),
		"copy lacks source read":  r.Copy(write, writeTarget, true),
		"move lacks write":        r.Move(read, target, true),
		"deny all":                r.Write(denied, nil, false),
		"deny foreign snapshot":   foreignErr,
	} {
		if !errors.Is(err, fs.ErrRootAccessDenied) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, err := range []error{r.Copy(read, writeTarget, true), r.Move(write, target, true)} {
		if !errors.Is(err, fs.ErrCrossRootOperation) {
			t.Errorf("cross-root pair reached dispatch: %v", err)
		}
	}
}

func TestOverlappingRootAccessUsesExactID(t *testing.T) {
	t.Parallel()
	a := t.TempDir()
	b := filepath.Join(a, "b")
	c := filepath.Join(b, "c")
	if err := os.MkdirAll(c, 0o700); err != nil {
		t.Fatal(err)
	}
	r := loadRegistry(t, filepath.Join(t.TempDir(), "config.toml"),
		"[files]\ndefault_id = 'a'\n"+
			rootTable(t, "a", a)+rootTable(t, "b", b)+"policy = 'writer'\n"+
			rootTable(t, "c", c)+"policy = 'writer'\naccess = ['files.read']\n"+
			rootTable(t, "blocked", c)+"policy = 'writer'\naccess = []\n"+
			"[policy.writer]\naccess = ['files.write']\n")
	viaA := fs.Operand{RootID: "a", Path: "b/c/file.txt"}
	viaB := fs.Operand{RootID: "b", Path: "c/file.txt"}
	viaC := fs.Operand{RootID: "c", Path: "file.txt"}
	viaBlocked := fs.Operand{RootID: "blocked", Path: "file.txt"}
	if err := r.Write(viaB, []byte("from B"), false); err != nil {
		t.Fatal(err)
	}
	for _, operand := range []fs.Operand{viaA, viaC} {
		if data, err := r.Read(operand, 64); err != nil || string(data) != "from B" {
			t.Errorf("read via %s = %q, %v", operand.RootID, data, err)
		}
		if err := r.Write(operand, []byte("forbidden"), true); !errors.Is(err, fs.ErrRootAccessDenied) {
			t.Errorf("write via %s = %v", operand.RootID, err)
		}
	}
	for _, operand := range []fs.Operand{viaB, viaBlocked} {
		if data, err := r.Read(operand, 64); !errors.Is(err, fs.ErrRootAccessDenied) || len(data) != 0 {
			t.Errorf("read via %s returned data or the wrong denial: %v", operand.RootID, err)
		}
	}
	if err := r.Delete(viaBlocked, false); !errors.Is(err, fs.ErrRootAccessDenied) {
		t.Errorf("deny-all alias allowed deletion: %v", err)
	}
	if err := r.Write(fs.Operand{RootID: "b", Path: "../outside.txt"}, nil, false); !errors.Is(err, security.ErrPathTraversal) {
		t.Errorf("write access bypassed path confinement: %v", err)
	}
	if made, err := r.Mkdir(fs.Operand{RootID: "b", Path: "c/new"}); err != nil || !made {
		t.Fatalf("write-only mkdir failed: %v", err)
	}
	from := fs.Operand{RootID: "b", Path: "c/new/seed.txt"}
	to := fs.Operand{RootID: "b", Path: "c/new/moved.txt"}
	testconfig.Write(t, filepath.Join(c, "new", "seed.txt"), []byte("seed"))
	if err := r.Move(from, to, false); err != nil {
		t.Fatalf("write-only move failed: %v", err)
	}
	if err := r.Delete(to, false); err != nil {
		t.Fatalf("write-only delete failed: %v", err)
	}
}

func TestProfileAndRiskCannotBeExpandedByRoot(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, profile string
	}{
		{"default profile", ""},
		{"read risk ceiling", "[profiles]\nactive = 'write'\nmax_risk = 'read'\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "seed.txt")
			testconfig.Write(t, source, []byte("seed"))
			registry := loadRegistryDocument(t, filepath.Join(t.TempDir(), "config.toml"),
				tc.profile+"[files]\ndefault_id = 'work'\n"+rootTable(t, "work", root)+
					"access = ['files.read', 'files.write', 'search.run']\n")
			file := fs.Operand{RootID: "work", Path: "seed.txt"}
			dest := fs.Operand{RootID: "work", Path: "new"}
			if data, err := registry.Read(file, 32); err != nil || string(data) != "seed" {
				t.Fatalf("permitted read failed: %v", err)
			}
			_, mkdirErr := registry.Mkdir(dest)
			for name, err := range map[string]error{
				"write":  registry.Write(file, []byte("denied"), true),
				"mkdir":  mkdirErr,
				"delete": registry.Delete(file, false),
				"copy":   registry.Copy(file, dest, false),
				"move":   registry.Move(file, dest, false),
			} {
				if !errors.Is(err, fs.ErrRootAccessDenied) {
					t.Errorf("%s should deny before dispatch: %v", name, err)
				}
			}
			if data, err := os.ReadFile(source); err != nil || string(data) != "seed" {
				t.Fatalf("denied operation affected the source: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, dest.Path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("denied operation affected the target: %v", err)
			}
		})
	}
}
