package config

import "testing"

func TestRootLinkPolicyComposition(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, fields string
		want         bool
	}{
		{"default", "", false},
		{"direct true", "roots.work.follow_symlinks=true\n", true},
		{"direct false", "roots.work.follow_symlinks=false\n", false},
		{"inherited true", "roots.work.policy='p'\n[policy.p]\nfollow_symlinks=true\n", true},
		{"inherited false", "roots.work.policy='p'\n[policy.p]\nfollow_symlinks=false\n", false},
		{"override false", "roots.work.policy='p'\nroots.work.follow_symlinks=false\n[policy.p]\nfollow_symlinks=true\n", false},
		{"override true", "roots.work.policy='p'\nroots.work.follow_symlinks=true\n[policy.p]\nfollow_symlinks=false\n", true},
		{"unused", "[policy.p]\nfollow_symlinks=true\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.fields), Direct)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := cfg.Files().Roots()
			if snapshot[0].FollowSymlinks != tc.want || snapshot[0].Access != RootRead {
				t.Fatal(snapshot)
			}
			snapshot[0].FollowSymlinks = !tc.want
			if cfg.Files().Roots()[0].FollowSymlinks != tc.want {
				t.Fatal("mutable snapshot")
			}
		})
	}
}

func TestRootLinkPolicyRejectsInvalidTypesAndGlobalAuthority(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, fields := range []string{
		"roots.work.follow_symlinks='true'\n", "roots.work.follow_symlinks=1\n",
		"roots.work.follow_symlinks=[]\n", "roots.work.follow_symlinks={}\n",
		"roots.work.Follow_Symlinks=true\n", "roots.work.follow_symlinks=true\nroots.work.follow_symlinks=false\n",
		"[policy.unused]\nfollow_symlinks='false'\n", "[policy.unused]\nfollow_symlinks=0\n",
		"[security]\nfollow_symlinks=false\n", "[security]\nfollow_symlinks=true\n",
		"follow_symlinks=true\n",
	} {
		_, err := Decode([]byte(root+fields), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
	}
}
