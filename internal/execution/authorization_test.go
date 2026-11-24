package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

func TestResourceFreeAuthorization(t *testing.T) {
	runtime, _, request := sessionFixture(t, nil)
	inspector := profileFixture(t, "active='inspector'\n[profiles.inspector]\ngrants=['system.read']\n")
	reader := profileFixture(t, "")
	for _, tc := range []struct {
		name    string
		profile config.EffectiveProfile
		allowed bool
	}{
		{"global right", inspector, true},
		{"missing global right", reader, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decl := fixtureDeclaration(t, tc.profile, capability.RiskRead, capability.SystemRead)
			routes := fixtureRoutes(decl)
			resolutions, calls, fallbacks := 0, 0, 0
			routes.Resolve = func(context.Context, probe) ([]Resource, error) {
				resolutions++
				return nil, nil
			}
			backend := probeBackend(func(_ context.Context, call Invocation[probe]) (Completion[string], error) {
				calls++
				binding := call.Binding()
				if len(binding.Resources()) != 0 || binding.Key() != decl.Key ||
					binding.Request().Caller().ID() != request.Caller().ID() ||
					binding.Profile().ID() != tc.profile.ID() {
					t.Fatal("resource-free authorization lost its operation or context")
				}
				return Complete(binding, "authorized result"), nil
			})
			fallback := probeBackend(func(context.Context, Invocation[probe]) (Completion[string], error) {
				fallbacks++
				return Completion[string]{}, nil
			})
			identity := Identity{Account: "effective", Binding: "process"}
			dispatcher, err := NewDispatcher(runtime, tc.profile, CurrentProcess, routes,
				Registration[probe, string]{ID: CurrentProcess, Identity: identity, Backend: backend},
				Registration[probe, string]{ID: UserWorker, Identity: identity, Backend: fallback})
			if err != nil {
				t.Fatal(err)
			}
			result, err := dispatcher.Dispatch(t.Context(), request, probe{}, nil)
			if tc.allowed {
				if err != nil || result != "authorized result" || calls != 1 || resolutions != 1 {
					t.Fatalf("allowed result = %q, error = %v, calls = %d, resolutions = %d", result, err, calls, resolutions)
				}
			} else if !errors.Is(err, ErrDenied) || result != "" || calls != 0 || resolutions != 0 {
				t.Fatalf("denied result = %q, error = %v, calls = %d, resolutions = %d", result, err, calls, resolutions)
			}
			if fallbacks != 0 {
				t.Fatalf("fallback calls = %d", fallbacks)
			}
		})
	}
}

func TestAuthorizerBindsOrderedOperandsAndDerivedRisk(t *testing.T) {
	runtime, _, request := sessionFixture(t, nil)
	for _, tc := range []struct {
		name     string
		settings string
		source   []capability.ID
		target   []capability.ID
		want     error
	}{
		{"both operands granted", "active='write'\n", []capability.ID{capability.FilesRead}, []capability.ID{capability.FilesWrite}, nil},
		{"source only denied", "active='write'\n", []capability.ID{capability.FilesWrite}, []capability.ID{capability.FilesWrite}, ErrDenied},
		{"target only denied", "active='write'\n", []capability.ID{capability.FilesRead}, []capability.ID{capability.FilesRead}, ErrDenied},
		{"operand raises risk", "active='write'\nmax_risk='read'\n", []capability.ID{capability.FilesRead}, []capability.ID{capability.FilesWrite}, ErrDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := profileFixture(t, tc.settings)
			decl := fixtureDeclaration(t, profile, capability.RiskRead, capability.FilesRead)
			catalog := profile.Catalog()
			write, err := catalog.NewRequirement(capability.FilesWrite)
			if err != nil {
				t.Fatal(err)
			}
			source, err := catalog.NewSet(tc.source...)
			if err != nil {
				t.Fatal(err)
			}
			target, err := catalog.NewSet(tc.target...)
			if err != nil {
				t.Fatal(err)
			}
			decl.Operands = []OperandRequirement{
				{Kind: Filesystem, Role: Source, Requirement: decl.Operation.Requirement()},
				{Kind: Filesystem, Role: Target, Requirement: write},
			}
			routes := fixtureRoutes(decl,
				Resource{Kind: Filesystem, Role: Source, ID: "work", Relative: "a", Access: source},
				Resource{Kind: Filesystem, Role: Target, ID: "work", Relative: "b", Access: target})
			calls := 0
			backend := probeBackend(func(_ context.Context, call Invocation[probe]) (Completion[string], error) {
				calls++
				binding := call.Binding()
				operands := binding.Resources()
				if binding.Operation().Risk() != capability.RiskMutation ||
					operands[0].Role != Source || !operands[0].Requirement.Requires(capability.FilesRead) ||
					operands[1].Role != Target || !operands[1].Requirement.Requires(capability.FilesWrite) {
					t.Fatal("combined risk or operand requirements differ from declaration")
				}
				return Complete(binding, "ok"), nil
			})
			dispatcher, err := NewDispatcher(runtime, profile, CurrentProcess, routes,
				Registration[probe, string]{ID: CurrentProcess, Identity: Identity{Account: "effective", Binding: "pid"}, Backend: backend})
			if err != nil {
				t.Fatal(err)
			}
			result, err := dispatcher.Dispatch(t.Context(), request, probe{}, nil)
			if tc.want == nil {
				if err != nil || result != "ok" || calls != 1 {
					t.Fatalf("result = %q, err = %v, calls = %d", result, err, calls)
				}
			} else if !errors.Is(err, tc.want) || result != "" || calls != 0 {
				t.Fatalf("result = %q, err = %v, calls = %d", result, err, calls)
			}
		})
	}
}

func TestAuthorizerRejectsInvalidDeclarations(t *testing.T) {
	runtime, _, _ := sessionFixture(t, nil)
	profile := profileFixture(t, "")
	foreign := profileFixture(t, "")
	local := fixtureDeclaration(t, profile, capability.RiskRead, capability.FilesRead)
	foreignDeclaration := fixtureDeclaration(t, foreign, capability.RiskRead, capability.FilesRead)
	missingRights := Declaration{Key: local.Key}
	missingRole := local
	missingRole.Operands = []OperandRequirement{{Kind: Filesystem, Requirement: local.Operation.Requirement()}}
	missingOperandRights := local
	missingOperandRights.Operands = []OperandRequirement{{Kind: Filesystem, Role: Single}}
	foreignOperand := local
	foreignOperand.Operands = []OperandRequirement{{Kind: Filesystem, Role: Single, Requirement: foreignDeclaration.Operation.Requirement()}}
	for name, declarations := range map[string][]Declaration{
		"missing": nil, "duplicate": {local, local}, "missing rights": {missingRights},
		"foreign": {foreignDeclaration}, "foreign operand": {foreignOperand}, "missing role": {missingRole},
		"missing operand rights": {missingOperandRights}, "missing key": {{}},
	} {
		t.Run(name, func(t *testing.T) {
			routes := fixtureRoutes(local)
			routes.Declarations = declarations
			if _, err := newAuthorizer(runtime, profile, routes); !errors.Is(err, ErrMismatch) {
				t.Fatalf("err = %v, want binding mismatch", err)
			}
		})
	}
}
