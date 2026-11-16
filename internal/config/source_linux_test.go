//go:build linux

package config

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestLinuxLocalAccountLookupAndServiceDefault(t *testing.T) {
	t.Parallel()
	const passwd = "operator:x:42:42::/home/operator:/bin/sh\nanother:x:43:43::/home/other:/bin/sh\n"
	home, err := linuxAccountHome([]byte(passwd), 42)
	if err != nil || home != "/home/operator" {
		t.Fatalf("unexpected account lookup: %v", err)
	}
	for _, table := range []string{passwd + passwd, "operator:x:42:42::relative:/bin/sh\n", ""} {
		if _, err := linuxAccountHome([]byte(table), 42); err == nil {
			t.Fatal("invalid account lookup succeeded")
		}
	}
	path, err := defaultPath(SystemService)
	if err != nil || path != "/etc/flashgate-mcp/flashgate_config.toml" {
		t.Fatalf("invalid Linux system-service default: %v", err)
	}
}

func TestLinuxLoadsOSReadableConfigAndRejectsSpecialFiles(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "owned")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(documentWithRoot(t, dir)), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(Source{Role: Direct, Path: path}); err != nil {
		t.Fatalf("OS-readable configuration failed to load: %v", err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(Source{Role: Direct, Path: fifo})
	assertCategory(t, err, CategoryUntrustedConfig)
}

func TestLinuxServiceOverrideUsesOSAccessAndRoleValidation(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("requires an ordinary Linux account")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	document := "[runtime]\nmode = \"service\"\n" + documentWithRoot(t, dir)
	if _, err := Decode([]byte(document), SystemService); err != nil {
		t.Fatalf("service document is invalid: %v", err)
	}
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(Source{Role: SystemService, Path: path}); err != nil {
		t.Fatalf("OS-readable explicit service source failed to load: %v", err)
	}
	_, err := Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryUnsupportedRuntime)
}

func TestLinuxConfigurationReaderDeniesSelectedPathExchange(t *testing.T) {
	dir := t.TempDir()
	for _, replace := range []bool{false, true} {
		path := filepath.Join(dir, "selected.toml")
		if err := os.WriteFile(path, []byte(documentWithRoot(t, dir)), 0o600); err != nil {
			t.Fatal(err)
		}
		err := withTrustedConfig(path, func(file *os.File) error {
			if _, err := readLinuxConfig(file); err != nil {
				return err
			}
			if err := os.Rename(path, filepath.Join(dir, "prior.toml")); err != nil {
				return err
			}
			if replace {
				return os.WriteFile(path, []byte(documentWithRoot(t, dir)), 0o600)
			}
			return nil
		})
		assertCategory(t, err, CategoryUntrustedConfig)
	}
}

func TestLinuxConfigurationDirectoryAccess(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires an unprivileged Linux account")
	}
	for _, tc := range []struct {
		name      string
		outerMode os.FileMode
		canSearch bool
	}{
		{"search without listing", 0o100, true},
		{"listing without search", 0o400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			outer := filepath.Join(root, "outer")
			inner := filepath.Join(outer, "inner")
			if err := os.MkdirAll(inner, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(inner, defaultFileName)
			if err := os.WriteFile(path, []byte(documentWithRoot(t, root)), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				for _, dir := range []string{outer, inner} {
					if err := os.Chmod(dir, 0o700); err != nil {
						t.Error(err)
					}
				}
			})
			if err := os.Chmod(inner, 0o100); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(outer, tc.outerMode); err != nil {
				t.Fatal(err)
			}
			_, readErr := os.ReadFile(path)
			if !tc.canSearch {
				if !errors.Is(readErr, os.ErrPermission) {
					t.Fatalf("test path unexpectedly traversable: %v", readErr)
				}
				_, err := Load(Source{Role: Direct, Path: path})
				assertCategory(t, err, CategoryUntrustedConfig)
				return
			}
			if readErr != nil {
				t.Fatalf("test file must be OS-readable: %v", readErr)
			}
			for _, dir := range []string{outer, inner} {
				if _, err := os.ReadDir(dir); !errors.Is(err, os.ErrPermission) {
					t.Fatalf("test directory must deny listing: %v", err)
				}
			}
			cfg, err := Load(Source{Role: Direct, Path: path})
			if err != nil {
				t.Fatalf("OS-readable config behind search-only parents failed to load: %v", err)
			}
			if cfg.Files().Roots()[0].Path != root {
				t.Fatal("loaded snapshot has an unexpected root")
			}
			guard, err := security.NewPathGuard(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := cfg.ProtectDataRoots([]*security.PathGuard{guard}); err != nil {
				t.Fatalf("configuration protection failed to bind: %v", err)
			}
		})
	}
}
