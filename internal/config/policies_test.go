package config

import (
	"slices"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

func TestNamedPolicyAccess(t *testing.T) {
	t.Parallel()
	const definitions = `
[policy.workspace]
access = ['files.read', 'files.write']
[policy.no_access]
access = []
[policy.defaults]
[policy.inert]
access = ['files.write']
`
	root := documentWithRoot(t, t.TempDir())
	read := []capability.ID{capability.FilesRead}
	write := []capability.ID{capability.FilesWrite}
	both := []capability.ID{capability.FilesRead, capability.FilesWrite}
	for _, tc := range []struct {
		name, settings string
		want           []capability.ID
	}{
		{"default despite unattached policies", "", read},
		{"explicit read", "work.access = ['files.read']\n", read},
		{"inherit", "work.policy = 'workspace'\n", both},
		{"replace", "work.policy = 'workspace'\nwork.access = ['files.write']\n", write},
		{"empty override", "work.policy = 'workspace'\nwork.access = []\n", nil},
		{"empty direct", "work.access = []\n", nil},
		{"empty inherited", "work.policy = 'no_access'\n", nil},
		{"missing policy field", "work.policy = 'defaults'\n", read},
		{"inline definition", "work.policy = 'inline'\n[policy]\ninline = { access = ['files.write'] }\n", write},
		{"known future root capability", "work.access = ['search.run', 'files.read']\n", []capability.ID{capability.FilesRead, capability.SearchRun}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.settings+definitions), Direct)
			if err != nil {
				t.Fatal(err)
			}
			want, err := cfg.Profile().Catalog().NewSet(tc.want...)
			if err != nil {
				t.Fatal(err)
			}
			got := cfg.Files().Roots()[0].Access
			if !got.Equal(want) || !slices.Equal(got.IDs(), tc.want) {
				t.Fatalf("effective rights = %v, want %v from the profile catalog", got.IDs(), tc.want)
			}
		})
	}
}

func TestNamedPolicyRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct{ name, settings string }{
		{"unknown reference", "work.policy = 'marker'\n"},
		{"unknown despite override", "work.policy = 'marker'\nwork.access = []\n"},
		{"case-sensitive reference", "work.policy = 'Marker'\n[policy.marker]\n"},
		{"empty reference", "work.policy = ''\n"},
		{"multiple references", "work.policy = ['marker']\n"},
		{"invalid name", "[policy.9marker]\n"},
		{"case alias", "[policy.Marker]\n"},
		{"unknown field", "[policy.marker]\npermissions = ['write']\n"},
		{"unknown table", "[policy.marker.unknown]\n"},
		{"recursive inheritance", "[policy.marker]\npolicy = 'marker'\n"},
		{"duplicate table", "[policy.marker]\n[policy.marker]\n"},
		{"duplicate field", "[policy.marker]\naccess = ['files.read']\naccess = ['files.write']\n"},
		{"conflicting definitions", "[policy]\nmarker = { access = [] }\n[policy.marker]\n"},
		{"duplicate reference", "work.policy = 'marker'\nwork.policy = 'marker'\n[policy.marker]\n"},
		{"duplicate root access", "work.access = ['files.read']\nwork.access = ['files.write']\n"},
		{"unreferenced invalid right", "[policy.marker]\naccess = ['marker']\n"},
		{"invalid policy field under override", "work.policy = 'marker'\nwork.access = ['files.read']\n[policy.marker]\naccess = ['marker']\n"},
		{"duplicate policy rights", "[policy.marker]\naccess = ['files.read', 'files.read']\n"},
		{"unknown root right", "work.access = ['marker']\n"},
		{"wrong case right", "work.access = ['Files.read']\n"},
		{"global-only right on root", "work.access = ['processes.read']\n"},
		{"global-only right on unused policy", "[policy.unused]\naccess = ['system.read']\n"},
		{"legacy root shorthand", "work.access = ['read']\n"},
		{"legacy named-policy shorthand", "[policy.marker]\naccess = ['write']\n"},
		{"duplicate root rights", "work.access = ['files.write', 'files.write']\n"},
		{"scalar access", "work.access = 'read'\n"},
		{"non-string right", "work.access = ['files.read', 1]\n"},
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
