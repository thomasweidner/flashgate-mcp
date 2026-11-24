package execution

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

type probe struct{ selection string }
type probeBackend func(context.Context, Invocation[probe]) (Completion[string], error)

func (f probeBackend) Execute(ctx context.Context, call Invocation[probe]) (Completion[string], error) {
	return f(ctx, call)
}

func profileFixture(t *testing.T, settings string) config.EffectiveProfile {
	t.Helper()
	path := strconv.Quote(t.TempDir())
	cfg, err := config.Decode([]byte("[files]\ndefault_id='work'\nwork.path="+path+"\n[profiles]\n"+settings), config.Direct)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Profile()
}

func sessionFixture(t *testing.T, groups []GroupID) (*Runtime, Session, Request) {
	t.Helper()
	runtime, err := NewRuntime("trusted-generation")
	if err != nil {
		t.Fatal(err)
	}
	session, err := newSession(runtime, "local-account", InheritedSTDIO, groups)
	if err != nil {
		t.Fatal(err)
	}
	request, err := session.Request(MCP20251125, 4096)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, session, request
}

func fixtureDeclaration(t *testing.T, profile config.EffectiveProfile, floor capability.Risk, ids ...capability.ID) Declaration {
	t.Helper()
	op, err := profile.Catalog().NewOperation(floor, ids...)
	if err != nil {
		t.Fatal(err)
	}
	return Declaration{Key: OperationKey{Name: "fixture.inspect", Variant: DefaultVariant}, Operation: op}
}

func fixtureRoutes(declaration Declaration, resources ...Resource) Routes[probe] {
	return Routes[probe]{
		Declarations: []Declaration{declaration},
		Key:          func(probe) OperationKey { return declaration.Key },
		Resolve:      func(context.Context, probe) ([]Resource, error) { return resources, nil },
	}
}

func TestCallerSnapshotAndRequestBinding(t *testing.T) {
	if _, err := NewRuntime(" "); !errors.Is(err, ErrMismatch) {
		t.Fatal("runtime accepted a missing generation", err)
	}
	groups := []GroupID{"z", "a", "a"}
	runtime, session, request := sessionFixture(t, groups)
	groups[0] = "forged"
	observed := request.Caller().Groups()
	observed[0] = "forged"
	if caller := request.Caller(); caller.ID() != "local-account" || caller.Origin() != InheritedSTDIO ||
		len(caller.Groups()) != 2 || caller.Groups()[0] != "a" || caller.Groups()[1] != "z" {
		t.Fatal("caller attributes lost their immutable binding")
	}
	if _, err := newSession(runtime, " ", InheritedSTDIO, nil); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	if _, err := newSession(runtime, "claimed", "payload", nil); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	if _, err := newSession(runtime, "claimed", InheritedSTDIO, []GroupID{""}); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	if _, err := session.Request("unsupported", 1); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	if _, err := (Session{}).Request(MCP20251125, 1); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	var ids sync.Map
	var group sync.WaitGroup
	for range 24 {
		group.Go(func() {
			next, err := session.Request(MCP20260728, 32)
			if err != nil || next.Protocol() != MCP20260728 || next.ResultBytes() != 32 {
				t.Error("wrong concurrent request", err)
			}
			if _, duplicate := ids.LoadOrStore(next.Correlation(), true); duplicate || next.Correlation() == request.Correlation() {
				t.Error("duplicate request correlation")
			}
		})
	}
	group.Wait()
}

func TestDispatcherRejectsMissingBackends(t *testing.T) {
	runtime, _, _ := sessionFixture(t, nil)
	profile := profileFixture(t, "")
	routes := fixtureRoutes(fixtureDeclaration(t, profile, capability.RiskRead, capability.FilesRead))
	identity := Identity{Account: "effective", Binding: "process"}
	var absent probeBackend
	noop := probeBackend(func(context.Context, Invocation[probe]) (Completion[string], error) {
		t.Fatal("unavailable backend executed")
		return Completion[string]{}, nil
	})
	valid := Registration[probe, string]{ID: CurrentProcess, Identity: identity, Backend: noop}
	for _, tc := range []struct {
		name     string
		selected BackendID
		entries  []Registration[probe, string]
		want     error
	}{
		{"missing", CurrentProcess, nil, ErrUnsupported},
		{"service absent", ServiceAccount, []Registration[probe, string]{valid}, ErrUnsupported},
		{"worker absent", UserWorker, []Registration[probe, string]{valid}, ErrUnsupported},
		{"unknown", "alias", []Registration[probe, string]{{ID: "alias", Identity: identity, Backend: noop}}, ErrUnsupported},
		{"duplicate", CurrentProcess, []Registration[probe, string]{valid, valid}, ErrMismatch},
		{"typed nil", CurrentProcess, []Registration[probe, string]{{ID: CurrentProcess, Identity: identity, Backend: absent}}, ErrMismatch},
		{"no identity", CurrentProcess, []Registration[probe, string]{{ID: CurrentProcess, Backend: noop}}, ErrMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if d, err := NewDispatcher(runtime, profile, tc.selected, routes, tc.entries...); d != nil || !errors.Is(err, tc.want) {
				t.Fatalf("dispatcher = %v, err = %v", d, err)
			}
		})
	}
}

func TestAuthorizationAndUnprovedEffectsDenyBeforeBackend(t *testing.T) {
	runtime, _, request := sessionFixture(t, nil)
	_, _, foreignRequest := sessionFixture(t, nil)
	read := profileFixture(t, "")
	write := profileFixture(t, "active='write'\n")
	readOnlyRisk := profileFixture(t, "active='write'\nmax_risk='read'\n")
	readDeclaration := fixtureDeclaration(t, read, capability.RiskRead, capability.FilesRead)
	readRoutes := fixtureRoutes(readDeclaration)
	unknown := readRoutes
	unknown.Key = func(probe) OperationKey { return OperationKey{Name: "fixture.unknown", Variant: DefaultVariant} }
	noVariant := readRoutes
	noVariant.Key = func(probe) OperationKey { return OperationKey{Name: readDeclaration.Key.Name} }
	readResource := Resource{Kind: Filesystem, Role: Source, ID: "work", Relative: "some-file", Access: write.Policy().Grants()}
	withOperand := readDeclaration
	withOperand.Operands = []OperandRequirement{{Kind: Filesystem, Role: Source, Requirement: readDeclaration.Operation.Requirement()}}
	cwd := readResource
	cwd.Kind = WorkingDirectory
	cwd.Access = read.Policy().Grants()
	withCWD := readDeclaration
	withCWD.Operands = []OperandRequirement{{Kind: WorkingDirectory, Role: Source, Requirement: readDeclaration.Operation.Requirement()}}
	provenCWD := cwd
	provenCWD.Proof = testProof{}
	identity := Identity{Account: "effective", Binding: "process"}
	reg := Registration[probe, string]{ID: CurrentProcess, Identity: identity, Backend: probeBackend(func(context.Context, Invocation[probe]) (Completion[string], error) {
		t.Fatal("denied request reached the backend")
		return Completion[string]{}, nil
	})}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	elapsed, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		request Request
		profile config.EffectiveProfile
		routes  Routes[probe]
		want    error
	}{
		{"zero request", t.Context(), Request{}, read, readRoutes, ErrMismatch},
		{"foreign generation", t.Context(), foreignRequest, read, readRoutes, ErrMismatch},
		{"canceled", canceled, request, read, readRoutes, context.Canceled},
		{"elapsed", elapsed, request, read, readRoutes, context.DeadlineExceeded},
		{"undeclared operation", t.Context(), request, read, unknown, ErrDenied},
		{"undeclared variant", t.Context(), request, read, noVariant, ErrDenied},
		{"partial copy grants", t.Context(), request, read, fixtureRoutes(fixtureDeclaration(t, read, capability.RiskRead, capability.FilesRead, capability.FilesWrite)), ErrDenied},
		{"risk floor", t.Context(), request, readOnlyRisk, fixtureRoutes(fixtureDeclaration(t, readOnlyRisk, capability.RiskMutation, capability.FilesRead)), ErrDenied},
		{"omitted operand", t.Context(), request, read, fixtureRoutes(withOperand), ErrDenied},
		{"foreign operand grants", t.Context(), request, read, fixtureRoutes(withOperand, readResource), ErrDenied},
		{"missing CWD proof", t.Context(), request, read, fixtureRoutes(withCWD, cwd), ErrUnsupported},
		{"missing CWD enforcement", t.Context(), request, read, fixtureRoutes(withCWD, provenCWD), ErrUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDispatcher(runtime, tc.profile, CurrentProcess, tc.routes, reg)
			if err != nil {
				t.Fatal(err)
			}
			if value, err := d.Dispatch(tc.ctx, tc.request, probe{}, nil); value != "" || !errors.Is(err, tc.want) {
				t.Fatalf("value = %q, err = %v", value, err)
			}
		})
	}
}

type testProof struct{}

func (testProof) Revalidate() error { return nil }

type directoryProof func() error

func (p directoryProof) Revalidate() error { return p() }

type cwdBackend struct{ probeBackend }

func (cwdBackend) ProveWorkingDirectory(ctx context.Context, binding *Binding) error {
	return binding.Verify(ctx, binding.Backend(), binding.EffectiveIdentity(), binding.Generation())
}

func TestBoundCWDRevalidatedBeforeEffectAndResult(t *testing.T) {
	runtime, _, request := sessionFixture(t, nil)
	profile := profileFixture(t, "")
	identity := Identity{Account: "effective", Binding: "fixture-pid"}
	declaration := fixtureDeclaration(t, profile, capability.RiskRead, capability.FilesRead)
	declaration.Operands = []OperandRequirement{{Kind: WorkingDirectory, Role: Single, Requirement: declaration.Operation.Requirement()}}
	drifted := false
	resource := Resource{Kind: WorkingDirectory, Role: Single, ID: "work", Relative: "cwd",
		Access: profile.Policy().Grants(),
		Proof: directoryProof(func() error {
			if drifted {
				return ErrMismatch
			}
			return nil
		})}
	backend := cwdBackend{func(ctx context.Context, invocation Invocation[probe]) (Completion[string], error) {
		deadline, _ := ctx.Deadline()
		binding := invocation.Binding()
		if deadline.IsZero() || !binding.Deadline().Equal(deadline) {
			t.Fatal("deadline did not reach the backend")
		}
		if err := invocation.AdmitEffect(ctx); err != nil {
			return Completion[string]{}, err
		}
		if invocation.Command().selection == "substituted" {
			drifted = true
		}
		return Complete(invocation.Binding(), "bound-value"), nil
	}}
	d, err := NewDispatcher(runtime, profile, CurrentProcess, fixtureRoutes(declaration, resource),
		Registration[probe, string]{ID: CurrentProcess, Identity: identity, Backend: backend})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	if value, err := d.Dispatch(ctx, request, probe{}, nil); err != nil || value != "bound-value" {
		t.Fatal("authorized CWD failed", err)
	}
	if value, err := d.Dispatch(ctx, request, probe{"substituted"}, nil); value != "" || !errors.Is(err, ErrMismatch) {
		t.Fatal("stale CWD disclosed completion", err)
	}
	if value, err := d.Dispatch(ctx, request, probe{}, func() error { t.Fatal("stale CWD reached effect"); return nil }); value != "" || !errors.Is(err, ErrMismatch) {
		t.Fatal("already stale CWD executed", err)
	}
}

func TestTypedDelegationCompletionAndNoFallback(t *testing.T) {
	runtime, _, request := sessionFixture(t, []GroupID{"reader"})
	profile := profileFixture(t, "")
	declaration := fixtureDeclaration(t, profile, capability.RiskRead, capability.FilesRead)
	identity := Identity{Account: "isolated-account", Binding: "test-worker"}
	var old *Binding
	worker := probeBackend(func(ctx context.Context, call Invocation[probe]) (Completion[string], error) {
		bind := call.Binding()
		if bind.Generation() != "trusted-generation" || bind.PolicyID() == "" || bind.Request().Correlation() != request.Correlation() ||
			bind.Request().Caller().ID() == identity.Account || bind.Request().ResultBytes() != 4096 {
			t.Fatal("wrong delegated context")
		}
		if !errors.Is(bind.Verify(ctx, CurrentProcess, identity, bind.Generation()), ErrMismatch) ||
			!errors.Is(bind.Verify(ctx, UserWorker, Identity{}, bind.Generation()), ErrMismatch) ||
			!errors.Is(bind.Verify(ctx, UserWorker, identity, "stale"), ErrMismatch) {
			t.Fatal("mismatched backend, account or generation accepted")
		}
		if err := bind.Verify(ctx, UserWorker, identity, bind.Generation()); err != nil {
			return Completion[string]{}, err
		}
		switch call.Command().selection {
		case "allowed":
			old = bind
			return Complete(bind, "typed-value"), nil
		case "describe":
			if bind.Key().Name != "fixture.describe" || bind.Operation().Risk() != capability.RiskRead {
				t.Fatal("new operation lost its own declaration")
			}
			return Complete(bind, "typed-description"), nil
		case "old":
			return Complete(old, "must not escape"), nil
		case "unbound":
			return Complete(nil, "must not escape"), nil
		default:
			return Complete(bind, "must not escape"), ErrUnsupported
		}
	})
	fallback := probeBackend(func(context.Context, Invocation[probe]) (Completion[string], error) {
		t.Fatal("automatic fallback executed")
		return Completion[string]{}, nil
	})
	describe := declaration
	describe.Key.Name = "fixture.describe"
	routes := fixtureRoutes(declaration)
	routes.Declarations = append(routes.Declarations, describe)
	routes.Key = func(command probe) OperationKey {
		if command.selection == "describe" {
			return describe.Key
		}
		return declaration.Key
	}
	d, err := NewDispatcher(runtime, profile, UserWorker, routes,
		Registration[probe, string]{ID: UserWorker, Identity: identity, Backend: worker},
		Registration[probe, string]{ID: CurrentProcess, Identity: Identity{Account: "local", Binding: "pid"}, Backend: fallback})
	if err != nil {
		t.Fatal(err)
	}
	if value, err := d.Dispatch(t.Context(), request, probe{"allowed"}, nil); err != nil || value != "typed-value" {
		t.Fatalf("value = %q, err = %v", value, err)
	}
	if value, err := d.Dispatch(t.Context(), request, probe{"describe"}, nil); err != nil || value != "typed-description" {
		t.Fatalf("new typed operation returned %q, %v", value, err)
	}
	for _, selection := range []string{"old", "unbound", "unsupported"} {
		if value, err := d.Dispatch(t.Context(), request, probe{selection}, nil); value != "" || err == nil {
			t.Fatalf("%s: value = %q, err = %v", selection, value, err)
		}
	}
	if value, err := d.Dispatch(t.Context(), request, probe{"allowed"}, func() error { return nil }); value != "" || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("delegated local admission = %q, %v", value, err)
	}
}
