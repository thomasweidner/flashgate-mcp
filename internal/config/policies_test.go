package config

import (
	"strings"
	"testing"
)

func TestNamedPolicyAccess(t *testing.T) {
	t.Parallel()
	const definitions = `
[policy.workspace]
access = ['read', 'write']
[policy.no_access]
access = []
[policy.defaults]
[policy.inert]
access = ['write']
`
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, settings string
		want           RootAccess
	}{
		{"default despite unattached policies", "", RootRead},
		{"explicit read", "roots.work.access = ['read']\n", RootRead},
		{"inherit", "roots.work.policy = 'workspace'\n", RootRead | RootWrite},
		{"replace", "roots.work.policy = 'workspace'\nroots.work.access = ['write']\n", RootWrite},
		{"empty override", "roots.work.policy = 'workspace'\nroots.work.access = []\n", 0},
		{"empty direct", "roots.work.access = []\n", 0},
		{"empty inherited", "roots.work.policy = 'no_access'\n", 0},
		{"missing policy field", "roots.work.policy = 'defaults'\n", RootRead},
		{"inline definition", "roots.work.policy = 'inline'\n[policy]\ninline = { access = ['write'] }\n", RootWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.settings+definitions), Direct)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Files().Roots()[0].Access; got != tc.want || got.Allows(0) {
				t.Fatalf("effective rights = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNamedPolicyRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct{ name, settings string }{
		{"unknown reference", "roots.work.policy = 'marker'\n"},
		{"unknown despite override", "roots.work.policy = 'marker'\nroots.work.access = []\n"},
		{"case-sensitive reference", "roots.work.policy = 'Marker'\n[policy.marker]\n"},
		{"empty reference", "roots.work.policy = ''\n"},
		{"multiple references", "roots.work.policy = ['marker']\n"},
		{"invalid name", "[policy.9marker]\n"},
		{"case alias", "[policy.Marker]\n"},
		{"unknown field", "[policy.marker]\npermissions = ['write']\n"},
		{"unknown table", "[policy.marker.unknown]\n"},
		{"recursive inheritance", "[policy.marker]\npolicy = 'marker'\n"},
		{"duplicate table", "[policy.marker]\n[policy.marker]\n"},
		{"duplicate field", "[policy.marker]\naccess = ['read']\naccess = ['write']\n"},
		{"conflicting definitions", "[policy]\nmarker = { access = [] }\n[policy.marker]\n"},
		{"duplicate reference", "roots.work.policy = 'marker'\nroots.work.policy = 'marker'\n[policy.marker]\n"},
		{"duplicate root access", "roots.work.access = ['read']\nroots.work.access = ['write']\n"},
		{"unreferenced invalid right", "[policy.marker]\naccess = ['marker']\n"},
		{"invalid policy field under override", "roots.work.policy = 'marker'\nroots.work.access = ['read']\n[policy.marker]\naccess = ['marker']\n"},
		{"duplicate policy rights", "[policy.marker]\naccess = ['read', 'read']\n"},
		{"unknown root right", "roots.work.access = ['marker']\n"},
		{"wrong case right", "roots.work.access = ['Read']\n"},
		{"duplicate root rights", "roots.work.access = ['write', 'write']\n"},
		{"scalar access", "roots.work.access = 'read'\n"},
		{"non-string right", "roots.work.access = ['read', 1]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(root+tc.settings), Direct)
			assertCategory(t, err, CategoryInvalidConfig)
			if strings.Contains(strings.ToLower(err.Error()), "marker") {
				t.Fatalf("error exposed config data: %v", err)
			}
		})
	}
}
