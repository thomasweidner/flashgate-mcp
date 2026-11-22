package roots

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestExclusionNativeWindowsCaseSensitiveDirectory(t *testing.T) {
	root := t.TempDir()
	sensitive := filepath.Join(root, "sensitive")
	if err := os.Mkdir(sensitive, 0o700); err != nil {
		t.Fatal(err)
	}
	fsutil := filepath.Join(os.Getenv("SystemRoot"), "System32", "fsutil.exe")
	output, err := exec.Command(fsutil, "file", "setCaseSensitiveInfo", sensitive, "enable").CombinedOutput()
	if err != nil || strings.Contains(string(output), "0x00000005") || strings.Contains(string(output), "0x00000032") {
		t.Skipf("native case-sensitive directory unavailable: %v (%s)", err, output)
	}
	insensitive, err := security.WindowsExclusionCase(sensitive)
	if err != nil || insensitive {
		t.Fatalf("case-sensitive directory did not classify: %v, %v", insensitive, err)
	}
	testconfig.Write(t, filepath.Join(sensitive, "lower.txt"), []byte("ordinary"))
	r := loadRegistry(t, filepath.Join(t.TempDir(), "active.toml"), "[files]\ndefault_id='work'\n"+
		rootTable(t, "work", root)+"excluded=['sensitive/LOWER.TXT']\n")
	if data, err := r.Read(fs.Operand{RootID: "work", Path: "sensitive/lower.txt"}, 40); err != nil || string(data) != "ordinary" {
		t.Fatalf("case-sensitive sibling was blocked: %v", err)
	}
	if _, err := r.Stat(fs.Operand{RootID: "work", Path: "sensitive/LOWER.TXT"}); !errors.Is(err, fs.ErrPathExcluded) {
		t.Fatalf("case-sensitive excluded spelling reached stat: %v", err)
	}
}
