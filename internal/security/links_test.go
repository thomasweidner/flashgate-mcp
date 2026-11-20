package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateChecksLinkPolicyBeforeTargetTraversal(t *testing.T) {
	root := t.TempDir()
	createSecurityTestSymlinkOrSkip(t, "cycle", filepath.Join(root, "cycle"))
	deny := mustNewPathGuard(t, root)
	if _, err := deny.ResolveForCreate("cycle/new.txt"); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("target traversal preceded policy", err)
	}
	allow := mustNewPathGuardWithPolicy(t, root, Policy{FollowSymlinks: true})
	if _, err := allow.ResolveForCreate("cycle/new.txt"); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("cycle was not bounded by policy", err)
	}
}

func TestRootAncestorLinkPolicyAndIdentity(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	data := filepath.Join(real, "data")
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	createSecurityTestSymlinkOrSkip(t, "real", alias)
	selected := filepath.Join(alias, "data")
	if _, err := NewPathGuard(selected); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("default policy followed an ancestor link", err)
	}
	g := mustNewPathGuardWithPolicy(t, selected, Policy{FollowSymlinks: true})
	if g.EffectiveRoot() != data {
		t.Fatal("wrong effective bootstrap root", g.EffectiveRoot())
	}
	info, err := PathIdentity(data)
	if err != nil || g.ValidateRootIdentity(info) != nil {
		t.Fatal("original native root identity rejected", err)
	}
	if err := os.Rename(alias, alias+".previous"); err != nil {
		t.Fatal(err)
	}
	createSecurityTestSymlinkOrSkip(t, "real", alias)
	if err := g.ValidateRootIdentity(info); !errors.Is(err, ErrOutsideRoot) {
		t.Fatal("new ancestor link admitted with the same target", err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := g.ValidateRootIdentity(info); err == nil {
		t.Fatal("missing root ancestor admitted")
	}
	createSecurityTestSymlinkOrSkip(t, real, alias)
	if _, err := NewPathGuardWithPolicy(selected, Policy{FollowSymlinks: true}); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("absolute ancestor link admitted", err)
	}
}

func TestStructuralRevalidationCannotSelectAnotherRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, ".host-owned", "data")
	if err := os.MkdirAll(filepath.Join(root, ".child"), 0700); err != nil {
		t.Fatal(err)
	}
	g := mustNewPathGuard(t, root)
	if _, err := g.ResolveExisting(".child"); !errors.Is(err, ErrHiddenPathDenied) {
		t.Fatal("hidden client selection admitted", err)
	}
	if _, err := g.RevalidateEffect(filepath.Join(root, ".child")); err != nil {
		t.Fatal("implicit structural descendant rejected", err)
	}
	for _, path := range []string{".child", base} {
		if _, err := g.RevalidateEffect(path); !errors.Is(err, ErrOutsideRoot) {
			t.Fatal("structural revalidation granted another selection", path, err)
		}
	}
	if _, err := g.LinkIdentities(base); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("link snapshot crossed its selected root", err)
	}
	info, err := PathIdentity(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.ValidateRootIdentity(info); !errors.Is(err, ErrOutsideRoot) {
		t.Fatal("different native root identity admitted", err)
	}
	if SamePolicyMetadata(nil, nil) || SamePolicyMetadata(info, nil) || SamePolicyMetadata(nil, info) {
		t.Fatal("missing metadata was treated as a policy observation")
	}
}

func TestRootAncestorLinkDepthIsBounded(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	target := "real"
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		createSecurityTestSymlinkOrSkip(t, target, filepath.Join(base, name))
		target = name
	}
	if _, err := NewPathGuardWithPolicy(filepath.Join(base, target), Policy{FollowSymlinks: true}); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("unbounded bootstrap link chain admitted", err)
	}
}

func TestLinkMetadataSnapshotsFailClosedWithoutGrantingAccess(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	g := mustNewPathGuardWithPolicy(t, root, Policy{FollowSymlinks: true})
	link := filepath.Join(root, "alias.txt")
	createSecurityTestSymlinkOrSkip(t, "file.txt", link)
	before, err := g.LinkIdentities(link)
	if err != nil || len(before) != 1 {
		t.Fatal("missing original link identity", before, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := g.LinkIdentities(link)
	if err != nil || SameLinks(before, after) {
		t.Fatal("link removal was invisible to the snapshot", err)
	}
	if _, err := RelativeLinkTarget(file); err == nil {
		t.Fatal("ordinary file classified as a relative link")
	}
	// Windows reports a missing child below a regular file as PATH_NOT_FOUND;
	// the snapshot may therefore be empty. It must never grant path access.
	if identities, err := g.LinkIdentities(filepath.Join(file, "child")); err == nil && len(identities) != 0 {
		t.Fatal("non-directory child acquired link authority", identities)
	}
	if _, err := g.ResolveExisting("file.txt/child"); err == nil {
		t.Fatal("metadata snapshot authorized a non-directory child")
	}
	cycle := filepath.Join(root, "cycle")
	createSecurityTestSymlinkOrSkip(t, "cycle", cycle)
	if _, err := g.LinkIdentities(cycle); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("metadata snapshot followed an unbounded cycle", err)
	}
	if _, err := EffectivePath(cycle); err == nil {
		t.Fatal("metadata resolution silently repaired a cycle")
	}
	absolute := filepath.Join(root, "absolute")
	createSecurityTestSymlinkOrSkip(t, file, absolute)
	if _, err := g.LinkIdentities(absolute); !errors.Is(err, ErrSymlinkDenied) {
		t.Fatal("metadata snapshot accepted an absolute link", err)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "retained" {
		t.Fatal("metadata checks affected file content", string(data), err)
	}
}
