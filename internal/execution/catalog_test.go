package execution

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

type catalogDomainBackend[C any] struct{}

func (catalogDomainBackend[C]) Execute(_ context.Context, invocation Invocation[C]) (Completion[string], error) {
	return Complete(invocation.Binding(), "allowed"), nil
}

func TestSharedCatalogDeclarationsAcrossDomainDispatchers(t *testing.T) {
	profile := profileFixture(t, "active='write'\n")
	inspect := fixtureDeclaration(t, profile, capability.RiskRead, capability.FilesRead)
	change := fixtureDeclaration(t, profile, capability.RiskMutation, capability.FilesWrite)
	change.Key.Name = "another-domain.change"
	declarations, err := BindDeclarations(profile.Catalog(), inspect, change)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BindDeclarations(profile.Catalog(), inspect, inspect); !errors.Is(err, ErrMismatch) {
		t.Fatalf("duplicate definitions accepted: %v", err)
	}
	_, session, _ := sessionFixture(t, nil)
	policy, request := fixturePolicy(t, session, profile)
	identity := Identity{Account: "fixture", Binding: "operator"}
	inspectRoutes := Routes[probe]{
		Shared: declarations, Key: func(probe) OperationKey { return inspect.Key },
		Resolve: func(context.Context, probe) ([]Resource, error) { return nil, nil },
	}
	inspector, err := NewDispatcher(policy, CurrentProcess, inspectRoutes,
		Registration[probe, string]{ID: CurrentProcess, Identity: identity, Backend: catalogDomainBackend[probe]{}, Operations: []OperationKey{inspect.Key}})
	if err != nil {
		t.Fatal(err)
	}
	writeRoutes := Routes[string]{
		Shared: declarations, Key: func(string) OperationKey { return change.Key },
		Resolve: func(context.Context, string) ([]Resource, error) { return nil, nil },
	}
	writeBackend := Registration[string, string]{ID: CurrentProcess, Identity: identity, Backend: catalogDomainBackend[string]{}, Operations: []OperationKey{change.Key}}
	writer, err := NewDispatcher(policy, CurrentProcess, writeRoutes, writeBackend)
	if err != nil {
		t.Fatal(err)
	}
	readView, err := inspector.CatalogContext(request)
	if err != nil {
		t.Fatal(err)
	}
	writeView, err := writer.CatalogContext(request)
	if err != nil || writeView.SameScope(readView) || !readView.Declared(change.Key) ||
		!writeView.Declared(inspect.Key) || readView.Eligible(change.Key) || writeView.Eligible(inspect.Key) {
		t.Fatalf("typed dispatchers do not consume one validated declaration source: %v", err)
	}
	if result, err := writer.Dispatch(t.Context(), request, "changed", nil); err != nil || result != "allowed" {
		t.Fatalf("shared declaration did not govern dispatch: %q, %v", result, err)
	}
	restricted, err := config.BindEffectiveProfile("write", profile.Catalog(), capability.RiskRead, capability.FilesRead, capability.FilesWrite)
	if err != nil {
		t.Fatal(err)
	}
	restrictedPolicy, restrictedRequest := fixturePolicy(t, session, restricted)
	restrictedWriter, err := NewDispatcher(restrictedPolicy, CurrentProcess, writeRoutes, writeBackend)
	if err != nil {
		t.Fatal(err)
	}
	restrictedView, err := restrictedWriter.CatalogContext(restrictedRequest)
	if err != nil || restrictedView.SameScope(writeView) || restrictedView.Eligible(change.Key) {
		t.Fatalf("shared declaration reused the admission from a different policy: %v", err)
	}
	if _, err := restrictedWriter.Dispatch(t.Context(), restrictedRequest, "", nil); !errors.Is(err, ErrDenied) {
		t.Fatalf("restricted view dispatched its hidden operation: %v", err)
	}
	for _, routes := range []map[CatalogRouteID]CatalogRoute{
		nil, {"inspect": {}}, {"inspect": inspector.CatalogRoute(), "foreign": restrictedWriter.CatalogRoute()},
	} {
		if _, err := BindCatalogRoutes(routes); !errors.Is(err, ErrMismatch) {
			t.Fatalf("invalid route composition accepted: %v", err)
		}
	}
	composition, err := BindCatalogRoutes(map[CatalogRouteID]CatalogRoute{"inspect": inspector.CatalogRoute(), "change": writer.CatalogRoute()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := composition.Context(restrictedRequest); !errors.Is(err, ErrMismatch) {
		t.Fatalf("wrong ingress assignment accepted: %v", err)
	}
	scope, err := composition.Context(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scope.BindInvocation("absent", []OperationKey{inspect.Key}); !errors.Is(err, ErrMismatch) {
		t.Fatalf("missing route accepted: %v", err)
	}
	if _, err := scope.BindInvocation("inspect", nil); !errors.Is(err, ErrMismatch) {
		t.Fatalf("empty operation set accepted: %v", err)
	}
	if _, err := scope.BindInvocation("inspect", []OperationKey{change.Key}); !errors.Is(err, ErrMismatch) {
		t.Fatalf("operation supported only by another route was accepted: %v", err)
	}
	invocation, err := scope.BindInvocation("inspect", []OperationKey{inspect.Key})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := composition.Context(invocation.Request()); !errors.Is(err, ErrMismatch) {
		t.Fatalf("restricted request could mint a new scope: %v", err)
	}
	ctx := invocation.Context(t.Context())
	if cleared := (CatalogInvocation{}).Context(ctx); cleared != ctx || invocation.Context(ctx) != ctx {
		t.Fatal("catalog invocation replaced an active context binding")
	}
	if _, err := inspector.Dispatch(ctx, request, probe{}, nil); !errors.Is(err, ErrMismatch) {
		t.Fatalf("unrestricted request replaced an active invocation: %v", err)
	}
}

func TestCatalogUsesDispatcherPolicyAndTrustedRequestScope(t *testing.T) {
	const inspect capability.ID = "fixture.inspect"
	catalog, err := capability.NewCatalog(append(capability.BaseDefinitions(),
		capability.Definition{ID: inspect, Description: "Inspect synthetic data.", MinimumRisk: capability.RiskRead})...)
	if err != nil {
		t.Fatal(err)
	}
	allow, err := config.BindEffectiveProfile("same", catalog, capability.RiskHigh, inspect, capability.FilesRead, capability.FilesWrite)
	if err != nil {
		t.Fatal(err)
	}
	read, err := config.BindEffectiveProfile("same", catalog, capability.RiskRead, inspect, capability.FilesRead)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := catalog.NewRequirement(capability.FilesWrite)
	if err != nil {
		t.Fatal(err)
	}
	base := fixtureDeclaration(t, allow, capability.RiskRead, inspect)
	if (CatalogContext{}).Declared(base.Key) || (CatalogContext{}).Eligible(base.Key) {
		t.Fatal("an unbound catalog context recognized an operation")
	}
	effect := fixtureDeclaration(t, allow, capability.RiskRead, inspect)
	effect.Key.Variant = "effect"
	effect.Operands = []OperandRequirement{
		{Kind: Filesystem, Role: Source, Requirement: base.Operation.Requirement()},
		{Kind: Filesystem, Role: Target, Requirement: pair},
	}
	high := fixtureDeclaration(t, allow, capability.RiskHigh, inspect)
	high.Key.Variant = "high"
	routes := fixtureRoutes(base)
	routes.Declarations = []Declaration{base, effect, high}
	routes.Key = func(command probe) OperationKey {
		if command.selection == "effect" {
			return effect.Key
		}
		return base.Key
	}
	runtime, session, unbound := sessionFixture(t, []GroupID{"reader"})
	allowPolicy, writeRequest := fixturePolicy(t, session, allow)
	readPolicy, readRequest := fixturePolicy(t, session, read)
	backend := probeBackend(func(_ context.Context, inv Invocation[probe]) (Completion[string], error) {
		return Complete(inv.Binding(), "allowed"), nil
	})
	newDispatcher := func(policy *PolicyBinding, selected BackendID, keys []OperationKey) *Dispatcher[probe, string] {
		t.Helper()
		dispatcher, err := NewDispatcher(policy, selected, routes,
			Registration[probe, string]{ID: selected, Identity: Identity{Account: "synthetic", Binding: "fixture"}, Backend: backend, Operations: keys})
		if err != nil {
			t.Fatal(err)
		}
		return dispatcher
	}
	writeDispatcher := newDispatcher(allowPolicy, ServiceAccount, routes.OperationKeys())
	readDispatcher := newDispatcher(readPolicy, CurrentProcess, routes.OperationKeys())
	writeScope, err := writeDispatcher.CatalogContext(writeRequest)
	if err != nil {
		t.Fatal(err)
	}
	readScope, err := readDispatcher.CatalogContext(readRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !writeScope.Eligible(base.Key) || !writeScope.Eligible(effect.Key) || !writeScope.Eligible(high.Key) ||
		!readScope.Eligible(base.Key) || readScope.Eligible(effect.Key) || readScope.Eligible(high.Key) ||
		writeScope.SameScope(readScope) {
		t.Fatal("effective views lost the shared rights, ordered resource requirements, risk, or policy identity")
	}
	for _, wrong := range []Request{unbound, readRequest} {
		if _, err := writeDispatcher.CatalogContext(wrong); !errors.Is(err, ErrMismatch) {
			t.Fatalf("unassigned or differently assigned request accepted: %v", err)
		}
		if _, err := writeDispatcher.Dispatch(t.Context(), wrong, probe{}, nil); !errors.Is(err, ErrMismatch) {
			t.Fatalf("wrong policy reached dispatch: %v", err)
		}
	}
	peer, err := newSession(runtime, "another-principal", InheritedSTDIO, []GroupID{"reader"})
	if err != nil {
		t.Fatal(err)
	}
	peer, err = peer.WithPolicy(readPolicy)
	if err != nil {
		t.Fatal(err)
	}
	peerRequest, err := peer.Request(MCP20251125, 4096)
	if err != nil {
		t.Fatal(err)
	}
	peerScope, err := readDispatcher.CatalogContext(peerRequest)
	if err != nil || peerScope.SameScope(readScope) || peerScope.Composition() != readScope.Composition() {
		t.Fatalf("different principal reused a catalog scope: %v", err)
	}
	otherRevision, err := readRequest.session.Request(MCP20260728, 4096)
	if err != nil {
		t.Fatal(err)
	}
	revisionScope, err := readDispatcher.CatalogContext(otherRevision)
	if err != nil || revisionScope.SameScope(readScope) {
		t.Fatalf("different protocol reused a catalog scope: %v", err)
	}
	_, _, foreign := sessionFixture(t, nil)
	if _, err := readDispatcher.CatalogContext(foreign); !errors.Is(err, ErrMismatch) {
		t.Fatalf("foreign generation accepted: %v", err)
	}
	narrow := newDispatcher(allowPolicy, ServiceAccount, []OperationKey{base.Key})
	narrowScope, err := narrow.CatalogContext(writeRequest)
	if err != nil || narrowScope.SameScope(writeScope) || !narrowScope.Eligible(base.Key) || narrowScope.Eligible(effect.Key) {
		t.Fatalf("backend support is not independently bound: %v", err)
	}
	if _, err := narrow.Dispatch(t.Context(), writeRequest, probe{"effect"}, nil); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported operation did not stop before resource resolution: %v", err)
	}
	var concurrent sync.WaitGroup
	for _, scope := range []CatalogContext{readScope, writeScope, peerScope, revisionScope} {
		concurrent.Go(func() {
			if !scope.Declared(effect.Key) || !scope.Eligible(base.Key) {
				t.Error("concurrent catalog read changed a definition")
			}
		})
	}
	concurrent.Wait()
}
