package roots

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestRootFileTypesExactIDAndConfigurationProtection(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.toml")
	r := loadRegistry(t, source, "[files]\ndefault_id='restricted'\n"+
		rootTable(t, "restricted", root)+"policy='text'\n"+
		rootTable(t, "unrestricted", root)+"access=['read','write']\n"+
		rootTable(t, "readonly", root)+"policy='text'\naccess=['read']\n"+
		"[policy.text]\naccess=['read','write']\nallowed_extensions=['.txt']\n")
	bad := filepath.Join(root, "bad.exe")
	if err := os.WriteFile(bad, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if b, err := r.Read(fs.Operand{RootID: "restricted", Path: "bad.exe"}, 100); !errors.Is(err, fs.ErrFileTypeDenied) || len(b) != 0 {
		t.Fatal(string(b), err)
	}
	if b, err := r.Read(fs.Operand{RootID: "unrestricted", Path: "bad.exe"}, 100); err != nil || string(b) != "bad" {
		t.Fatal(string(b), err)
	}
	if err := r.Write(fs.Operand{RootID: "readonly", Path: "new.txt"}, nil, false); !errors.Is(err, fs.ErrRootAccessDenied) {
		t.Fatal(err)
	}
	if err := r.Copy(fs.Operand{RootID: "unrestricted", Path: "bad.exe"}, fs.Operand{RootID: "restricted", Path: "new.txt"}, false); !errors.Is(err, fs.ErrCrossRootOperation) {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "protected.txt")
	if err := os.Link(source, alias); err != nil {
		t.Fatal(err)
	}
	for _, id := range []config.RootID{"restricted", "unrestricted"} {
		p := fs.Operand{RootID: id, Path: "protected.txt"}
		if b, err := r.Read(p, 65536); !errors.Is(err, security.ErrConfigurationAccess) || len(b) != 0 {
			t.Fatal(id, string(b), err)
		}
		if err := r.Copy(p, fs.Operand{RootID: id, Path: "output.txt"}, false); !errors.Is(err, security.ErrConfigurationAccess) {
			t.Fatal(err)
		}
		if err := r.Write(p, []byte("replace"), true); !errors.Is(err, security.ErrConfigurationAccess) {
			t.Fatal(err)
		}
	}
	if _, err := r.Stat(fs.Operand{RootID: "restricted", Path: "protected.txt"}); err != nil {
		t.Fatal("allowed content-independent metadata", err)
	}
	if _, err := r.Stat(fs.Operand{RootID: "restricted", Path: "config.toml"}); !errors.Is(err, fs.ErrFileTypeDenied) {
		t.Fatal("type-denied metadata", err)
	}
	if _, err := os.Stat(filepath.Join(root, "output.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("protected content exported", err)
	}
}
