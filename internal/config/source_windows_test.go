//go:build windows

package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestWindowsConfigPaths(t *testing.T) {
	t.Parallel()
	path, err := defaultPath(SystemService)
	if err != nil || path != "C:\\ProgramData\\FlashGate\\flashgate_config.toml" {
		t.Fatalf("invalid Windows service default: %v", err)
	}
	for _, path := range []string{
		"C:relative.toml", "\\\\server\\share\\config.toml", "\\\\?\\C:\\config.toml",
		"C:\\folder\\..\\config.toml", "C:\\config.toml:secret", "C:\\CON.toml",
		"C:\\folder.\\config.toml", "C:\\",
	} {
		if _, _, ok := splitWindowsPath(path); ok {
			t.Fatalf("unsafe Windows source accepted: %q", path)
		}
	}
	if volume, parts, ok := splitWindowsPath("C:/A Profile/flashgate_config.toml"); !ok || volume != "C:" || len(parts) != 2 {
		t.Fatal("ordinary absolute Windows path rejected")
	}
}

func TestWindowsRejectsJunction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, filepath.Join(target, "config.toml"), []byte(documentWithRoot(t, target)))
	junction := filepath.Join(dir, "junction")
	cmd := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("junction fixture unavailable: %v %s", err, output)
	}
	_, err := Load(Source{Role: Direct, Path: filepath.Join(junction, "config.toml")})
	assertCategory(t, err, CategoryUntrustedConfig)
}
func TestWindowsConfigurationSharingAndRoles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	testconfig.Write(t, path, []byte("[runtime]\nmode = \"service\"\n"+documentWithRoot(t, dir)))
	if _, err := Load(Source{Role: SystemService, Path: path}); err != nil {
		t.Fatalf("OS-readable service configuration did not load: %v", err)
	}
	_, err := Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryUnsupportedRuntime)

	err = withTrustedConfig(path, func(*os.File) error {
		writer, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err == nil {
			_ = writer.Close()
			t.Error("writer opened during the configuration read")
		} else if !errors.Is(err, syscall.Errno(32)) {
			t.Errorf("expected sharing violation, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
