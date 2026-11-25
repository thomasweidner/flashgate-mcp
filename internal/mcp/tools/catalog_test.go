package tools

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type catalogFixtureBackend struct{}

func (catalogFixtureBackend) Execute(_ context.Context, invocation execution.Invocation[execution.VariantID]) (execution.Completion[struct{}], error) {
	return execution.Complete(invocation.Binding(), struct{}{}), nil
}

func TestCatalogRejectsMalformedRegistrations(t *testing.T) {
	bind := func(handlers.Context) (execution.CatalogContext, error) {
		return execution.CatalogContext{}, execution.ErrMismatch
	}
	handle := func(handlers.Context, execution.CatalogInvocation, json.RawMessage) (ToolResult, error) {
		t.Fatal("composition must not invoke a handler")
		return ToolResult{}, nil
	}
	validate := func(*CatalogView) error { return execution.ErrMismatch }
	tool := &testTool{name: "fixture", inputSchema: map[string]any{"type": "object"}}
	key := execution.OperationKey{Name: "fixture.inspect", Variant: execution.DefaultVariant}
	representation := CatalogRepresentation{
		Support: CatalogSupport{
			Revision: execution.MCP20251125, Backend: execution.CurrentProcess,
			Origin: execution.InheritedSTDIO, Platform: runtime.GOOS,
		},
		Metadata: tool, Variants: []execution.OperationKey{key}, Outcomes: []CatalogOutcome{{key, defaultVariant}},
	}
	var absent *testTool
	noMetadata := representation
	noMetadata.Metadata = absent
	noSchema := representation
	noSchema.Metadata = &testTool{name: tool.name}
	invalidSchema := representation
	invalidSchema.Metadata = &testTool{name: tool.name, inputSchema: func() {}}
	wrongName := representation
	wrongName.Support.Revision = execution.MCP20260728
	wrongName.Metadata = &testTool{name: "another_fixture", inputSchema: tool.inputSchema}
	duplicateVariant := representation
	duplicateVariant.Variants = []execution.OperationKey{key, key}
	noOutcomes := representation
	noOutcomes.Outcomes = nil
	wrongOutcome := representation
	wrongOutcome.Outcomes = []CatalogOutcome{{key, "unknown"}}
	duplicateOutcome := representation
	duplicateOutcome.Outcomes = []CatalogOutcome{{key, defaultVariant}, {key, defaultVariant}}
	for _, tc := range []struct {
		name string
		reps []CatalogRepresentation
	}{
		{"empty", nil},
		{"typed nil metadata", []CatalogRepresentation{noMetadata}},
		{"missing input schema", []CatalogRepresentation{noSchema}},
		{"unencodable input schema", []CatalogRepresentation{invalidSchema}},
		{"another name in another revision", []CatalogRepresentation{representation, wrongName}},
		{"duplicate variant", []CatalogRepresentation{duplicateVariant}},
		{"missing outcomes", []CatalogRepresentation{noOutcomes}},
		{"unknown result", []CatalogRepresentation{wrongOutcome}},
		{"duplicate outcome", []CatalogRepresentation{duplicateOutcome}},
		{"duplicate support scope", []CatalogRepresentation{representation, representation}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if catalog, err := NewCatalog(bind, validate, &CatalogTool{Representations: tc.reps, Handle: handle}); err == nil || catalog != nil {
				t.Fatalf("invalid definition produced a catalog: %v", err)
			}
		})
	}
	definition := &CatalogTool{Representations: []CatalogRepresentation{representation}, Handle: handle}
	if _, err := NewCatalog(nil, validate, definition); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("composition accepted a missing request binder: %v", err)
	}
	if _, err := NewCatalog(bind, nil, definition); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("composition accepted a missing activation validator: %v", err)
	}
}

func TestCatalogAdmissionAndInvocationShareExactViews(t *testing.T) {
	const inspect capability.ID = "fixture.inspect"
	reviewed, err := capability.NewCatalog(append(capability.BaseDefinitions(),
		capability.Definition{ID: inspect, Description: "Inspect synthetic data.", MinimumRisk: capability.RiskRead})...)
	if err != nil {
		t.Fatal(err)
	}
	baseOperation, err := reviewed.NewOperation(capability.RiskRead, inspect)
	if err != nil {
		t.Fatal(err)
	}
	highOperation, err := reviewed.NewOperation(capability.RiskHigh, inspect, capability.FilesRead)
	if err != nil {
		t.Fatal(err)
	}
	base := execution.OperationKey{Name: "fixture.inspect", Variant: execution.DefaultVariant}
	high := execution.OperationKey{Name: base.Name, Variant: "detail"}
	internal := execution.OperationKey{Name: "fixture.internal", Variant: execution.DefaultVariant}
	routes := execution.Routes[execution.VariantID]{
		Declarations: []execution.Declaration{
			{Key: base, Operation: baseOperation}, {Key: high, Operation: highOperation},
			{Key: internal, Operation: baseOperation},
		},
		Key: func(variant execution.VariantID) execution.OperationKey {
			return execution.OperationKey{Name: base.Name, Variant: variant}
		},
		Resolve: func(context.Context, execution.VariantID) ([]execution.Resource, error) { return nil, nil },
	}
	type fixtureID int
	type fixtureKey struct{}
	runtimes := make(map[fixtureID]func(execution.Protocol) (execution.CatalogContext, error))
	dispatchers := make(map[fixtureID]*execution.Dispatcher[execution.VariantID, struct{}])
	lifecycle, err := execution.NewRuntime("fixture-generation")
	if err != nil {
		t.Fatal(err)
	}
	for id, settings := range []struct {
		ceiling        capability.Risk
		representation execution.RepresentationID
		grants         []capability.ID
		operations     []execution.OperationKey
	}{
		{capability.RiskRead, "", []capability.ID{inspect}, []execution.OperationKey{base}},
		{capability.RiskHigh, "native", []capability.ID{inspect, capability.FilesRead}, []execution.OperationKey{base, high}},
		{capability.RiskHigh, "native", []capability.ID{inspect, capability.FilesRead}, []execution.OperationKey{base}},
		{capability.RiskRead, "", []capability.ID{capability.FilesRead}, []execution.OperationKey{base}},
		{capability.RiskHigh, "native", []capability.ID{inspect, capability.FilesRead}, []execution.OperationKey{base}},
	} {
		profile, err := config.BindEffectiveProfile("same", reviewed, settings.ceiling, settings.grants...)
		if err != nil {
			t.Fatal(err)
		}
		session, identity, err := execution.BindDirectSTDIO(lifecycle)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := execution.NewPolicyBinding(lifecycle, profile)
		if err != nil {
			t.Fatal(err)
		}
		session, err = session.WithPolicy(policy)
		if err != nil {
			t.Fatal(err)
		}
		dispatcher, err := execution.NewDispatcher(policy, execution.CurrentProcess, routes,
			execution.Registration[execution.VariantID, struct{}]{
				ID: execution.CurrentProcess, Identity: identity, Backend: catalogFixtureBackend{},
				Representation: settings.representation, Operations: settings.operations,
			})
		if err != nil {
			t.Fatal(err)
		}
		dispatchers[fixtureID(id)] = dispatcher
		runtimes[fixtureID(id)] = func(revision execution.Protocol) (execution.CatalogContext, error) {
			request, err := session.Request(revision, 4096)
			if err != nil {
				return execution.CatalogContext{}, err
			}
			return dispatcher.CatalogContext(request)
		}
	}
	request := func(id fixtureID, revision execution.Protocol) handlers.Context {
		return handlers.Context{Context: context.WithValue(t.Context(), fixtureKey{}, id), ProtocolVersion: string(revision)}
	}
	bind := func(ctx handlers.Context) (execution.CatalogContext, error) {
		id, ok := ctx.Context.Value(fixtureKey{}).(fixtureID)
		if !ok || runtimes[id] == nil {
			return execution.CatalogContext{}, execution.ErrMismatch
		}
		return runtimes[id](execution.Protocol(ctx.ProtocolVersion))
	}
	support := func(revision execution.Protocol, representation execution.RepresentationID) CatalogSupport {
		return CatalogSupport{Revision: revision, Backend: execution.CurrentProcess, Origin: execution.InheritedSTDIO, Platform: runtime.GOOS, Representation: representation}
	}
	inputSchema := map[string]any{"type": "object"}
	metadata := &testTool{name: "fixture_inspect", description: "Current representation", inputSchema: inputSchema}
	native := &testTool{name: metadata.name, description: "Native adapter representation", inputSchema: inputSchema}
	newer := &testTool{name: metadata.name, description: "Candidate representation", inputSchema: inputSchema}
	var calls atomic.Int32
	baseOutcome := []CatalogOutcome{{base, defaultVariant}}
	bothOutcomes := []CatalogOutcome{{base, defaultVariant}, {high, defaultVariant}}
	tool := &CatalogTool{
		Representations: []CatalogRepresentation{
			{Support: support(execution.MCP20251125, ""), Metadata: metadata, Variants: []execution.OperationKey{base, high}, Outcomes: bothOutcomes},
			{Support: support(execution.MCP20251125, "native"), Metadata: native, Variants: []execution.OperationKey{base, high}, Outcomes: bothOutcomes},
			{Support: support(execution.MCP20260728, ""), Metadata: newer, Variants: []execution.OperationKey{base}, Outcomes: baseOutcome},
		},
		Handle: func(ctx handlers.Context, binding execution.CatalogInvocation, args json.RawMessage) (ToolResult, error) {
			if !binding.Allows(base) {
				t.Error("handler received a variant outside its effective view")
			}
			key := base
			if string(args) == `{"detail":true}` {
				key = high
			}
			if !binding.Allows(key) {
				return ToolResult{}, execution.ErrUnsupported
			}
			id, _ := ctx.Context.Value(fixtureKey{}).(fixtureID)
			if _, err := dispatchers[id].Dispatch(ctx.Context, binding.Request(), key.Variant, nil); err != nil {
				return ToolResult{}, err
			}
			calls.Add(1)
			return NewToolResult(defaultVariant, createDirectoryResult{Path: "fixture", Created: true}), nil
		},
	}
	validate := func(view *CatalogView) error {
		if !slices.Contains(view.OperationKeys(metadata.name), base) {
			return execution.ErrMismatch
		}
		return nil
	}
	registry, err := NewCatalog(bind, validate, tool, tool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCatalog(bind, validate, tool, &CatalogTool{Representations: tool.Representations, Handle: tool.Handle}); err == nil {
		t.Fatal("accepted an independent definition for the same name")
	}
	inputSchema["type"] = "mutated"
	read := request(0, execution.MCP20251125)
	write := request(1, execution.MCP20251125)
	candidate := request(0, execution.MCP20260728)
	limited := request(4, execution.MCP20251125)
	for i, ctx := range []handlers.Context{read, write, candidate, limited} {
		view, err := registry.PrepareView(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := []execution.OperationKey{base}
		if i == 1 {
			want = append(want, high)
		}
		if len(view.List()) != 1 || !slices.Equal(view.OperationKeys(metadata.name), want) {
			t.Fatal("wrong authorized operation inventory")
		}
	}
	if view, err := registry.PrepareView(request(3, execution.MCP20251125)); err != nil || len(view.List()) != 0 {
		t.Fatalf("technical support admitted a tool without its functional grant: %v", err)
	}
	call := NewCallHandler(registry)
	args := json.RawMessage(`{"name":"fixture_inspect"}`)
	if _, rpcErr := call.Handle(read, args); rpcErr == nil {
		t.Fatal("call succeeded before exact view admission")
	}
	if len(registry.List()) != 0 {
		t.Fatal("unadmitted catalog definitions leaked through the registry")
	}
	if _, ok := registry.Get(metadata.name); ok {
		t.Fatal("unadmitted definition could be resolved")
	}
	if err := registry.Activate(read, read); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("duplicate views were admitted: %v", err)
	}
	other, err := NewCatalog(bind, func(*CatalogView) error { return execution.ErrDenied }, tool)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Activate(read); !errors.Is(err, execution.ErrDenied) || len(other.List()) != 0 {
		t.Fatalf("unvalidated composition was admitted: %v", err)
	}
	if err := registry.Activate(read, write, candidate, limited); err != nil {
		t.Fatal(err)
	}
	if err := registry.Activate(read, write, candidate, limited); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("a sealed admission could be replaced: %v", err)
	}
	for _, ctx := range []handlers.Context{read, write, candidate, limited} {
		if _, rpcErr := call.Handle(ctx, args); rpcErr != nil {
			t.Fatal(rpcErr)
		}
	}
	if calls.Load() != 4 {
		t.Fatal("direct admitted calls reached the wrong handler")
	}
	detail := json.RawMessage(`{"name":"fixture_inspect","arguments":{"detail":true}}`)
	if _, err := call.Handle(limited, detail); err == nil || calls.Load() != 4 {
		t.Fatal("optional variant executed without backend support")
	}
	if _, err := call.Handle(write, detail); err != nil || calls.Load() != 5 {
		t.Fatalf("proved optional variant did not reach its selected backend: %v", err)
	}
	stale := request(2, execution.MCP20251125)
	if view, err := registry.PrepareView(stale); err != nil || !slices.Equal(view.OperationKeys(metadata.name), []execution.OperationKey{base}) {
		t.Fatalf("optional operation selected by backend type alone: %v", err)
	}
	if _, rpcErr := call.Handle(stale, args); rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatal("a fresh but unadmitted generation resolved a tool")
	}
	currentList, rpcErr := NewListHandler(registry).Handle(read, nil)
	actual, err := json.Marshal(currentList)
	if err != nil || rpcErr != nil || !strings.Contains(string(actual), "Current representation") ||
		!strings.Contains(string(actual), `"type":"object"`) || strings.Contains(string(actual), "mutated") {
		t.Fatalf("discovery lost immutable representation bytes: %s, %v, %v", actual, rpcErr, err)
	}
	if _, err := registry.PrepareView(request(1, execution.MCP20260728)); err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := call.Handle(request(1, execution.MCP20260728), args); rpcErr == nil {
		t.Fatal("unsupported backend and revision resolved a direct call")
	}
	unknown := execution.OperationKey{Name: "fixture.unknown", Variant: execution.DefaultVariant}
	broken := &CatalogTool{
		Handle: tool.Handle, Representations: []CatalogRepresentation{
			{Support: support(execution.MCP20251125, ""), Metadata: metadata,
				Variants: []execution.OperationKey{unknown}, Outcomes: []CatalogOutcome{{unknown, defaultVariant}}},
		},
	}
	invalid, err := NewCatalog(bind, validate, broken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invalid.PrepareView(read); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("unknown operation acquired an effective view: %v", err)
	}
}
