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

func TestProcessWorkingDirectoryPolicy(t *testing.T) {
	t.Parallel()
	const definitions = "[policy.allowed]\nallow_process_cwd=true\nfollow_symlinks=true\n" +
		"[policy.denied]\nallow_process_cwd=false\n"
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, settings string
		want, follow   bool
	}{
		{name: "default"},
		{name: "direct deny", settings: "work.allow_process_cwd=false\n"},
		{name: "direct allow", settings: "work.allow_process_cwd=true\n", want: true},
		{name: "inherit allow", settings: "work.policy='allowed'\n", want: true, follow: true},
		{name: "inherit deny", settings: "work.policy='denied'\n"},
		{name: "override allow", settings: "work.policy='denied'\nwork.allow_process_cwd=true\n", want: true},
		{name: "override deny", settings: "work.policy='allowed'\nwork.allow_process_cwd=false\n", follow: true},
		{name: "independent link override", settings: "work.policy='allowed'\nwork.follow_symlinks=false\n", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.settings+definitions), Direct)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := cfg.Files().Roots()
			if snapshot[0].AllowProcessCWD != tc.want || snapshot[0].FollowSymlinks != tc.follow ||
				!slices.Equal(snapshot[0].Access.IDs(), []capability.ID{capability.FilesRead}) {
				t.Fatalf("wrong effective CWD, link or access policy: %+v", snapshot[0])
			}
			snapshot[0].AllowProcessCWD = !tc.want
			if cfg.Files().Roots()[0].AllowProcessCWD != tc.want {
				t.Fatal("caller modified the effective CWD setting")
			}
		})
	}
	for _, value := range []string{"1", "0.0", "'true'", "[true]", "{ enabled=true }"} {
		t.Run(value, func(t *testing.T) {
			for _, prefix := range []string{"work.allow_process_cwd=", "[policy.unused]\nallow_process_cwd=",
				"work.policy='selected'\nwork.allow_process_cwd=false\n[policy.selected]\nallow_process_cwd="} {
				_, err := Decode([]byte(root+prefix+value+"\n"), Direct)
				assertCategory(t, err, CategoryInvalidConfig)
			}
		})
	}
	for _, invalid := range []string{"work.allow_process_workdir=true\n", "[policy.unused]\nallow_process_workdir=true\n",
		"work.allow_process_cwd=true\nwork.allow_process_cwd=false\n", "allow_process_cwd=true\n", "[security]\nallow_process_cwd=true\n"} {
		_, err := Decode([]byte(root+invalid), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
	}
}
