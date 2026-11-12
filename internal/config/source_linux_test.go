//go:build linux

package config

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
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

func TestLinuxRejectsWritableFileAndParentAndSpecialFiles(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "owned")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(documentWithRoot(t, dir)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o622); err != nil {
		t.Fatal(err)
	}
	_, err := Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryUntrustedConfig)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o722); err != nil {
		t.Fatal(err)
	}
	_, err = Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryUntrustedConfig)
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(Source{Role: Direct, Path: fifo})
	assertCategory(t, err, CategoryUntrustedConfig)
}

func TestLinuxOwnerTrustIsRoleBound(t *testing.T) {
	t.Parallel()
	metadata := syscall.Stat_t{Mode: syscall.S_IFREG | 0o600, Uid: 42, Nlink: 1}
	if !trustedLinuxNode(&metadata, 42, false, false) || trustedLinuxNode(&metadata, 42, true, false) ||
		trustedLinuxNode(&metadata, 43, false, false) {
		t.Fatal("wrong identity or role accepted")
	}
	metadata.Uid = 0
	if !trustedLinuxNode(&metadata, 42, true, false) || trustedLinuxNode(&metadata, 42, false, false) {
		t.Fatal("system provisioning was not isolated from user ownership")
	}
}

func TestLinuxSystemRoleCannotLoadUserOwnedConfig(t *testing.T) {
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
	_, err := Load(Source{Role: SystemService, Path: path})
	assertCategory(t, err, CategoryUntrustedConfig)
}
