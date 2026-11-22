package config

import "testing"

func TestRootLinkPolicyComposition(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, fields string
		want         bool
	}{
		{"default", "", false},
		{"direct true", "work.follow_symlinks=true\n", true},
		{"direct false", "work.follow_symlinks=false\n", false},
		{"inherited true", "work.policy='p'\n[policy.p]\nfollow_symlinks=true\n", true},
		{"inherited false", "work.policy='p'\n[policy.p]\nfollow_symlinks=false\n", false},
		{"override false", "work.policy='p'\nwork.follow_symlinks=false\n[policy.p]\nfollow_symlinks=true\n", false},
		{"override true", "work.policy='p'\nwork.follow_symlinks=true\n[policy.p]\nfollow_symlinks=false\n", true},
		{"unused", "[policy.p]\nfollow_symlinks=true\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.fields), Direct)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := cfg.Files().Roots()
			if snapshot[0].FollowSymlinks != tc.want || !snapshot[0].Access.Equal(cfg.Profile().Policy().Grants()) {
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
		"work.follow_symlinks='true'\n", "work.follow_symlinks=1\n",
		"work.follow_symlinks=[]\n", "work.follow_symlinks={}\n",
		"work.Follow_Symlinks=true\n", "work.follow_symlinks=true\nwork.follow_symlinks=false\n",
		"[policy.unused]\nfollow_symlinks='false'\n", "[policy.unused]\nfollow_symlinks=0\n",
		"[security]\nfollow_symlinks=false\n", "[security]\nfollow_symlinks=true\n",
		"follow_symlinks=true\n",
	} {
		_, err := Decode([]byte(root+fields), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
	}
}
