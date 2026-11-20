package roots

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestRootLinkPolicyMixedOverlappingIDs(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.txt")
	r := loadRegistry(t, source, "[files]\ndefault_id='allow'\n"+
		rootTable(t, "allow", root)+"policy='p'\nallowed_extensions=['.txt']\n"+
		rootTable(t, "deny", root)+"policy='p'\nfollow_symlinks=false\n"+
		rootTable(t, "default", root)+"access=['read','write']\n"+
		"[policy.p]\naccess=['read','write']\nfollow_symlinks=true\n")
	if err := os.WriteFile(filepath.Join(root, "ordinary.txt"), []byte("ordinary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("ordinary.txt", filepath.Join(root, "alias.txt")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := os.Symlink("config.txt", filepath.Join(root, "protected.txt")); err != nil {
		t.Fatal(err)
	}
	for _, id := range r.IDs() {
		p := fs.Operand{RootID: id, Path: "alias.txt"}
		b, err := r.Read(p, 100)
		if id == "allow" {
			if err != nil || string(b) != "ordinary" {
				t.Fatal(string(b), err)
			}
		} else if !errors.Is(err, security.ErrSymlinkDenied) {
			t.Fatal("other root inherited a grant", id, err)
		}
		protected := fs.Operand{RootID: id, Path: "protected.txt"}
		if _, err := r.Read(protected, 10000); err == nil {
			t.Fatal("protected source read", id)
		}
		if err := r.Copy(protected, fs.Operand{RootID: id, Path: "output.txt"}, false); err == nil {
			t.Fatal("protected copy source", id)
		}
		if err := r.Write(protected, []byte("changed"), true); err == nil {
			t.Fatal("protected mutation", id)
		}
		if b, err := r.Read(fs.Operand{RootID: id, Path: "ordinary.txt"}, 100); err != nil || string(b) != "ordinary" {
			t.Fatal("sibling denied", id, err)
		}
	}
	if err := r.Copy(fs.Operand{RootID: "allow", Path: "alias.txt"}, fs.Operand{RootID: "deny", Path: "output.txt"}, false); !errors.Is(err, fs.ErrCrossRootOperation) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "output.txt")); !os.IsNotExist(err) {
		t.Fatal("denied output exists", err)
	}
}
