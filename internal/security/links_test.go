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

func TestLinkAliasParentUsesActualTraversalAndSnapshots(t *testing.T) {
	for _, tc := range []struct {
		kind        string
		targetAlias bool
	}{
		{"absolute", false}, {"hidden", false}, {"allowed", false},
		{"absolute", true}, {"hidden", true}, {"allowed", true},
	} {
		name := tc.kind + "/parent-alias"
		if tc.targetAlias {
			name = tc.kind + "/target-alias"
		}
		t.Run(name, func(t *testing.T) {
			kind := tc.kind
			root := t.TempDir()
			real := filepath.Join(root, "real")
			if err := os.MkdirAll(filepath.Join(real, "deep"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"secret.txt", "real/.hidden.txt", "real/target.txt"} {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			createSecurityTestSymlinkOrSkip(t, filepath.Join("real", "deep"), filepath.Join(root, "alias"))
			name := filepath.Join("alias", "leaf.txt")
			if tc.targetAlias {
				name = "leaf.txt"
				createSecurityTestSymlinkOrSkip(t, "alias"+string(filepath.Separator)+".."+string(filepath.Separator)+"secret.txt", filepath.Join(root, name))
			} else {
				createSecurityTestSymlinkOrSkip(t, filepath.Join("..", "secret.txt"), filepath.Join(real, "deep", "leaf.txt"))
			}
			target := "target.txt"
			if kind == "hidden" {
				target = ".hidden.txt"
			} else if kind == "absolute" {
				target = filepath.Join(real, ".hidden.txt")
			}
			selected := filepath.Join(real, "secret.txt")
			createSecurityTestSymlinkOrSkip(t, target, selected)
			g := mustNewPathGuardWithPolicy(t, root, Policy{FollowSymlinks: true})
			for _, resolve := range []func(string) (SafePath, error){g.ResolveExisting, g.ResolveForCreate} {
				_, err := resolve(name)
				if (err == nil) != (kind == "allowed") {
					t.Fatalf("actual target policy: kind=%s err=%v", kind, err)
				}
			}
			parent, err := g.ResolveExisting(filepath.Dir(name))
			if err != nil {
				t.Fatal(err)
			}
			listed, err := g.AllowListEntry(parent, "leaf.txt")
			if err != nil || listed != (kind == "allowed") {
				t.Fatal("list chain policy", listed, err)
			}
			before, err := g.LinkIdentities(filepath.Join(root, name))
			if kind == "absolute" {
				if !errors.Is(err, ErrSymlinkDenied) {
					t.Fatal("snapshot skipped absolute target link", err)
				}
				return
			}
			if err != nil || len(before) != 3 || !HasLeafLink(filepath.Join(root, name), before) {
				t.Fatal("incomplete actual chain snapshot", len(before), err)
			}
			if err := os.Rename(selected, selected+".previous"); err != nil {
				t.Fatal(err)
			}
			createSecurityTestSymlinkOrSkip(t, target, selected)
			after, err := g.LinkIdentities(filepath.Join(root, name))
			if err != nil || SameLinks(before, after) {
				t.Fatal("actual target-link replacement was invisible", err)
			}
		})
	}
}

func TestRootAliasParentUsesActualBootstrapChain(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		t.Run(map[bool]string{false: "relative", true: "absolute"}[absolute], func(t *testing.T) {
			base := t.TempDir()
			real := filepath.Join(base, "real")
			for _, dir := range []string{"selected", "real/data", "real/deep"} {
				if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(dir)), 0700); err != nil {
					t.Fatal(err)
				}
			}
			createSecurityTestSymlinkOrSkip(t, filepath.Join("real", "deep"), filepath.Join(base, "alias"))
			createSecurityTestSymlinkOrSkip(t, filepath.Join("..", "selected"), filepath.Join(real, "deep", "root-link"))
			target := "data"
			if absolute {
				target = filepath.Join(real, "data")
			}
			selected := filepath.Join(real, "selected")
			createSecurityTestSymlinkOrSkip(t, target, selected)
			g, err := NewPathGuardWithPolicy(filepath.Join(base, "alias", "root-link"), Policy{FollowSymlinks: true})
			if absolute {
				if !errors.Is(err, ErrSymlinkDenied) {
					t.Fatal("bootstrap skipped absolute actual-parent target", err)
				}
				return
			}
			root := filepath.Join(base, "alias", "root-link")
			effective, effectiveErr := evalEffectivePath(root)
			if effectiveErr != nil || effective != filepath.Join(real, "data") {
				t.Fatal("metadata resolution selected the decoy", effective, effectiveErr)
			}
			// Some Windows kernels refuse this nested relative root chain.
			// That must deny bootstrap, never substitute the lexical decoy.
			info, nativeErr := PathIdentity(root)
			if nativeErr != nil {
				if g != nil || !errors.Is(err, ErrOutsideRoot) {
					t.Fatal("bootstrap admitted a root without native identity", err, nativeErr)
				}
				t.Log("native root handle unavailable; bootstrap denied:", nativeErr)
			} else if err != nil || g.EffectiveRoot() != effective || g.ValidateRootIdentity(info) != nil {
				t.Fatal("valid native bootstrap chain rejected", err)
			}
			volume := filepath.VolumeName(root) + string(filepath.Separator)
			before, err := snapshotLinks(volume, root, nil)
			if err != nil || len(before) != 3 {
				t.Fatal("incomplete bootstrap chain snapshot", len(before), err)
			}
			if err := os.Rename(selected, selected+".previous"); err != nil {
				t.Fatal(err)
			}
			createSecurityTestSymlinkOrSkip(t, target, selected)
			after, err := snapshotLinks(volume, root, nil)
			if err != nil || SameLinks(before, after) {
				t.Fatal("actual bootstrap link replacement was invisible", err)
			}
			if g != nil && !errors.Is(g.ValidateRootIdentity(info), ErrOutsideRoot) {
				t.Fatal("actual bootstrap chain replacement admitted", err)
			}
		})
	}
}

func TestExclusionTraversalRejectsConfiguredRootLinkDrift(t *testing.T) {
	for _, tc := range []struct {
		name     string
		absolute bool
	}{
		{name: "replacement with the same target"},
		{name: "unproved absolute substitute", absolute: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "real")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(base, "alias")
			createSecurityTestSymlinkOrSkip(t, "real", alias)
			guard := mustNewPathGuardWithPolicy(t, alias, Policy{FollowSymlinks: true})
			operand, err := guard.ResolveForCreate("new.txt")
			if err != nil {
				t.Fatal(err)
			}
			// Retain the original entry so a recycled file identity cannot
			// turn the replacement into an apparent match.
			if err := os.Rename(alias, filepath.Join(base, "original-alias")); err != nil {
				t.Fatal(err)
			}
			target := "real"
			if tc.absolute {
				target = root
			}
			createSecurityTestSymlinkOrSkip(t, target, alias)
			links, visited, err := guard.ExclusionTraversal(operand.String())
			if len(links) != 0 || len(visited) != 0 || !errors.Is(err, ErrOutsideRoot) {
				t.Fatalf("changed root link supplied a traversal: %v, %v, %v", links, visited, err)
			}
		})
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
