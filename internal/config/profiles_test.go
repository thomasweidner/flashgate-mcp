package config

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

func TestResolveProfiles(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	read := []capability.ID{capability.FilesRead}
	write := []capability.ID{capability.FilesRead, capability.FilesWrite}
	const extra = "[profiles.definitions.extra]\ngrants = ['files.write']\n"
	for _, tc := range []struct {
		name, options string
		id            ProfileID
		grants        []capability.ID
		ceiling       capability.Risk
	}{
		{"omitted", extra, ProfileRead, read, capability.RiskMutation},
		{"write", "[profiles]\nactive = 'write'\n" + extra, ProfileWrite, write, capability.RiskMutation},
		{"custom", "[profiles]\nactive = 'dev'\n[profiles.definitions.dev]\ngrants = ['files.write', 'files.read']\n", "dev", write, capability.RiskMutation},
		{"explicit composition", "[profiles]\nactive = 'read'\ninclude = ['extra']\n" + extra, ProfileRead, write, capability.RiskMutation},
		{"tight ceiling", "[profiles]\nactive = 'write'\nmax_risk = 'read'\n", ProfileWrite, write, capability.RiskRead},
		{"high ceiling with default grants", "[profiles]\nmax_risk = 'high'\n", ProfileRead, read, capability.RiskHigh},
		{"explicit high-risk grant", "[profiles]\nactive = 'ops'\nmax_risk = 'high'\n[profiles.definitions.ops]\ngrants = ['processes.control']\n", "ops", []capability.ID{capability.ProcessesControl}, capability.RiskHigh},
		{"empty grants", "[profiles]\nactive = 'empty'\n[profiles.definitions.empty]\ngrants = []\n", "empty", nil, capability.RiskMutation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.options), Direct)
			if err != nil {
				t.Fatal(err)
			}
			profile := cfg.Profile()
			if profile.ID() != tc.id || profile.Policy().Ceiling() != tc.ceiling || !slices.Equal(profile.Policy().Grants().IDs(), tc.grants) {
				t.Fatalf("unexpected profile %q and grants %v", profile.ID(), profile.Policy().Grants().IDs())
			}
			var readers sync.WaitGroup
			for range 8 {
				readers.Go(func() {
					ids := cfg.Profile().Policy().Grants().IDs()
					if !slices.Equal(ids, tc.grants) {
						t.Error("effective grants changed")
					}
					clear(ids)
				})
			}
			readers.Wait()
		})
	}
}

func TestRejectInvalidProfiles(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, options string
		category      ErrorCategory
	}{
		{"unknown active", "[profiles]\nactive = 'admin'\n", CategoryInvalidProfile},
		{"empty active", "[profiles]\nactive = ''\n", CategoryInvalidProfile},
		{"invalid active type", "[profiles]\nactive = 1\n", CategoryInvalidConfig},
		{"missing grants", "[profiles.definitions.dev]\n", CategoryInvalidProfile},
		{"invalid grants type", "[profiles.definitions.dev]\ngrants = 'files.read'\n", CategoryInvalidConfig},
		{"mis-cased name", "[profiles.definitions.Dev]\ngrants = []\n", CategoryInvalidConfig},
		{"bad name", "[profiles.definitions.1bad]\ngrants = []\n", CategoryInvalidProfile},
		{"duplicate definition", "[profiles.definitions.dev]\ngrants = []\n[profiles.definitions.dev]\ngrants = []\n", CategoryInvalidConfig},
		{"builtin override", "[profiles.definitions.read]\ngrants = []\n", CategoryInvalidProfile},
		{"wildcard", "[profiles.definitions.dev]\ngrants = ['files.*']\n", CategoryInvalidProfile},
		{"mixed case grant", "[profiles.definitions.dev]\ngrants = ['Files.read']\n", CategoryInvalidProfile},
		{"duplicate grant", "[profiles.definitions.dev]\ngrants = ['files.read', 'files.read']\n", CategoryInvalidProfile},
		{"unknown grant", "[profiles.definitions.dev]\ngrants = ['files.own']\n", CategoryInvalidProfile},
		{"implicit composition", "[profiles]\ninclude = ['write']\n", CategoryInvalidProfile},
		{"duplicate component", "[profiles]\nactive = 'read'\ninclude = ['write','write']\n", CategoryInvalidProfile},
		{"self component", "[profiles]\nactive = 'read'\ninclude = ['read']\n", CategoryInvalidProfile},
		{"unknown component", "[profiles]\nactive = 'read'\ninclude = ['other']\n", CategoryInvalidProfile},
		{"nested composition", "[profiles.definitions.dev]\ngrants = []\ninclude = ['read']\n", CategoryInvalidConfig},
		{"unknown risk ceiling", "[profiles]\nmax_risk = 'critical'\n", CategoryInvalidProfile},
		{"mis-cased risk ceiling", "[profiles]\nmax_risk = 'High'\n", CategoryInvalidProfile},
		{"declared risk", "[profiles.definitions.dev]\ngrants = []\nrisk = 'read'\n", CategoryInvalidConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertProfileError(t, root+tc.options, tc.category)
		})
	}
	for _, old := range []string{
		"filesystem.read", "filesystem.write", "search.execute", "process.observe",
		"process.manage", "process.control.external", "command.execute",
	} {
		t.Run(old, func(t *testing.T) {
			document := root + fmt.Sprintf("[profiles.definitions.dev]\ngrants = ['files.read', '%s']\n", old)
			assertProfileError(t, document, CategoryInvalidProfile)
		})
	}
}

func assertProfileError(t *testing.T, document string, want ErrorCategory) {
	t.Helper()
	cfg, err := Decode([]byte(document), Direct)
	if category, ok := CategoryOf(err); !ok || category != want || cfg.Profile().ID() != "" {
		t.Fatalf("got %v, want %v", err, want)
	}
}
