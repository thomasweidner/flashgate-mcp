//go:build windows || linux

package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedAuditTreeAndHardlinkAlias(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, "server.toml")
	if err := os.WriteFile(settings, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := PathIdentity(settings)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := BindConfigProtection(settings, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if err := protection.ProtectAuditTree(root); !errors.Is(err, ErrProtectedPath) {
		t.Fatalf("audit storage could take over configuration parent: %v", err)
	}
	store := filepath.Join(root, "reserved", "audit")
	if err := protection.ProtectAuditTree(store); err != nil {
		t.Fatal(err)
	}
	guard, err := NewPathGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := protection.BindRoot(guard, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.CheckAudit(filepath.Join(store, "missing"), nil, nil); !errors.Is(err, ErrAuditPath) {
		t.Fatalf("future protected namespace accessible: %v", err)
	}
	dir := filepath.Join(store, "2026", "11", "25")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "audit-000001.jsonl")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.jsonl")
	if err := os.Link(file, alias); err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := PathIdentity(alias)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := PathIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.CheckAudit(alias, aliasInfo, parent); !errors.Is(err, ErrAuditPath) {
		t.Fatalf("hardlink exposed audit data: %v", err)
	}
	ordinary := filepath.Join(root, "normal.txt")
	if err := os.WriteFile(ordinary, []byte("ordinary fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	ordinaryAlias := filepath.Join(root, "normal-alias.txt")
	if err := os.Link(ordinary, ordinaryAlias); err != nil {
		t.Fatal(err)
	}
	ordinaryInfo, err := PathIdentity(ordinaryAlias)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.CheckAudit(ordinaryAlias, ordinaryInfo, parent); err != nil {
		t.Fatalf("ordinary unrelated hardlink was denied: %v", err)
	}
	storeRoot, err := os.OpenRoot(store)
	if err != nil {
		t.Fatal(err)
	}
	defer storeRoot.Close()
	remaining := 1
	if err := checkAuditIdentity(storeRoot, ".", 0, ordinaryInfo, &remaining); !errors.Is(err, ErrAuditPath) {
		t.Fatalf("identity verification exceeded its count bound: %v", err)
	}
	reserved, err := PathIdentity(filepath.Dir(store))
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.Check(filepath.Dir(store), reserved, parent, true); !errors.Is(err, ErrAuditPath) {
		t.Fatalf("audit ancestor mutation accepted: %v", err)
	}
}
