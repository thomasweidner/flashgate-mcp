package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

type syntheticBackend[C, R any] struct {
	value   R
	effects *atomic.Int32
}

func (b syntheticBackend[C, R]) Execute(_ context.Context, invocation execution.Invocation[C]) (execution.Completion[R], error) {
	b.effects.Add(1)
	return execution.Complete(invocation.Binding(), b.value), nil
}

type syntheticStatFilesystem struct{ fs.FileSystem }

func (syntheticStatFilesystem) Stat(fs.Operand) (fs.Metadata, error) {
	return fs.Metadata{Name: "fixture"}, nil
}

func (syntheticStatFilesystem) Write(path fs.Operand, _ []byte, _ bool) error {
	return path.AdmitEffect()
}

func (syntheticStatFilesystem) LimitsForRoot(config.RootID) (resource.Limits, error) {
	return fs.DefaultLimits().ResourceLimits(), nil
}

func TestBL110IndependentPrincipalsUseOneEffectiveMaster(t *testing.T) {
	catalog, err := capability.NewCatalog(capability.BaseDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	readKey := execution.OperationKey{Name: "fixture.observe", Variant: execution.DefaultVariant}
	optionalKey := execution.OperationKey{Name: readKey.Name, Variant: "optional"}
	writeKey := execution.OperationKey{Name: "fixture.change", Variant: execution.DefaultVariant}
	readOperation, err := catalog.NewOperation(capability.RiskRead, capability.FilesRead)
	if err != nil {
		t.Fatal(err)
	}
	writeOperation, err := catalog.NewOperation(capability.RiskMutation, capability.FilesRead, capability.FilesWrite)
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := execution.BindDeclarations(catalog,
		execution.Declaration{Key: readKey, Operation: readOperation},
		execution.Declaration{Key: optionalKey, Operation: readOperation},
		execution.Declaration{Key: writeKey, Operation: writeOperation})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := execution.NewRuntime("shared-generation")
	if err != nil {
		t.Fatal(err)
	}
	type changeCommand struct{ key execution.OperationKey }
	type subject struct {
		session    execution.Session
		policy     *execution.PolicyBinding
		dispatcher *execution.Dispatcher[execution.OperationKey, struct{}]
		change     *execution.Dispatcher[changeCommand, string]
		decoy      *execution.Dispatcher[execution.OperationKey, struct{}]
		catalog    *execution.CatalogRoutes
	}
	subjects := make(map[execution.PrincipalID]subject)
	var readEffects, writeEffects, wrongEffects, wrongResolutions atomic.Int32
	for i, id := range []execution.PrincipalID{"fixture-reader", "fixture-writer", "fixture-unadmitted"} {
		grants := []capability.ID{capability.FilesRead}
		ceiling := capability.RiskRead
		if i == 1 {
			grants = append(grants, capability.FilesWrite)
			ceiling = capability.RiskMutation
		}
		profile, err := config.BindEffectiveProfile("same", catalog, ceiling, grants...)
		if err != nil {
			t.Fatal(err)
		}
		session, err := execution.BindCatalogFixtureSession(owner, id, execution.InheritedSTDIO, nil)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := execution.NewPolicyBinding(owner, profile)
		if err != nil {
			t.Fatal(err)
		}
		session, err = session.WithPolicy(policy)
		if err != nil {
			t.Fatal(err)
		}
		readRoutes := execution.Routes[execution.OperationKey]{
			Shared:  definitions,
			Key:     func(key execution.OperationKey) execution.OperationKey { return key },
			Resolve: func(context.Context, execution.OperationKey) ([]execution.Resource, error) { return nil, nil },
		}
		registeredRead := execution.Registration[execution.OperationKey, struct{}]{
			ID: execution.CurrentProcess, Identity: execution.Identity{Account: "fixture-operator", Binding: "shared"},
			Backend: syntheticBackend[execution.OperationKey, struct{}]{effects: &readEffects}, Operations: []execution.OperationKey{readKey},
		}
		dispatcher, err := execution.NewDispatcher(policy, execution.CurrentProcess, readRoutes, registeredRead)
		if err != nil {
			t.Fatal(err)
		}
		change, err := execution.NewDispatcher(policy, execution.CurrentProcess, execution.Routes[changeCommand]{
			Shared:  definitions,
			Key:     func(command changeCommand) execution.OperationKey { return command.key },
			Resolve: func(context.Context, changeCommand) ([]execution.Resource, error) { return nil, nil },
		}, execution.Registration[changeCommand, string]{
			ID: execution.CurrentProcess, Identity: registeredRead.Identity, Operations: []execution.OperationKey{writeKey},
			Backend: syntheticBackend[changeCommand, string]{value: "changed", effects: &writeEffects},
		})
		if err != nil {
			t.Fatal(err)
		}
		readRoutes.Resolve = func(context.Context, execution.OperationKey) ([]execution.Resource, error) {
			wrongResolutions.Add(1)
			return nil, nil
		}
		registeredRead.Backend = syntheticBackend[execution.OperationKey, struct{}]{effects: &wrongEffects}
		registeredRead.Operations = []execution.OperationKey{readKey, optionalKey}
		decoy, err := execution.NewDispatcher(policy, execution.CurrentProcess, readRoutes, registeredRead)
		if err != nil {
			t.Fatal(err)
		}
		routes := map[execution.CatalogRouteID]execution.CatalogRoute{"inspect": dispatcher.CatalogRoute(), "change": change.CatalogRoute()}
		composition, err := execution.BindCatalogRoutes(routes)
		if err != nil {
			t.Fatal(err)
		}
		routes["inspect"] = decoy.CatalogRoute()
		subjects[id] = subject{session: session, policy: policy, dispatcher: dispatcher, change: change, decoy: decoy, catalog: composition}
	}
	type principalKey struct{}
	request := func(subject execution.PrincipalID) handlers.Context {
		return handlers.Context{
			Context: context.WithValue(t.Context(), principalKey{}, subject), ProtocolVersion: string(execution.MCP20251125),
		}
	}
	bind := func(ctx handlers.Context) (execution.CatalogContext, error) {
		id, _ := ctx.Context.Value(principalKey{}).(execution.PrincipalID)
		subject, known := subjects[id]
		if !known {
			return execution.CatalogContext{}, execution.ErrMismatch
		}
		request, err := subject.session.Request(execution.Protocol(ctx.ProtocolVersion), 4096)
		if err != nil {
			return execution.CatalogContext{}, err
		}
		return subject.catalog.Context(request)
	}
	var calls atomic.Int32
	trusted := tools.CatalogSupport{Revision: execution.MCP20251125, Backend: execution.CurrentProcess,
		Origin: execution.InheritedSTDIO, Platform: runtime.GOOS}
	unsupported := trusted
	unsupported.Platform = "fixture-unsupported-platform"
	register := func(route execution.CatalogRouteID, metadata tools.Tool, support tools.CatalogSupport, keys ...execution.OperationKey) *tools.CatalogTool {
		var outcomes []tools.CatalogOutcome
		for _, key := range keys {
			for _, result := range metadata.ResultContract() {
				outcomes = append(outcomes, tools.CatalogOutcome{Operation: key, Result: result.Name})
			}
		}
		return &tools.CatalogTool{
			Route:           route,
			Representations: []tools.CatalogRepresentation{{Support: support, Metadata: metadata, Variants: keys, Outcomes: outcomes}},
			Handle: func(ctx handlers.Context, scope execution.CatalogInvocation, arguments json.RawMessage) (tools.ToolResult, error) {
				request := scope.Request()
				bound := subjects[request.Caller().ID()]
				switch {
				case route == "change":
					value, err := bound.change.Dispatch(ctx.Context, request, changeCommand{writeKey}, nil)
					if err != nil || value != "changed" {
						return tools.ToolResult{}, execution.ErrMismatch
					}
				case string(arguments) == `{"wrong":true}`:
					_, err := bound.decoy.Dispatch(ctx.Context, request, readKey, nil)
					return tools.ToolResult{}, err
				case string(arguments) == `{"unbound":true}`:
					unbound, err := bound.session.Request(request.Protocol(), 4096)
					if err != nil {
						return tools.ToolResult{}, err
					}
					_, err = bound.decoy.Dispatch(ctx.Context, unbound, readKey, nil)
					return tools.ToolResult{}, err
				default:
					key := readKey
					if string(arguments) == `{"optional":true}` {
						key = optionalKey
					}
					if _, err := bound.dispatcher.Dispatch(ctx.Context, request, key, nil); err != nil {
						return tools.ToolResult{}, err
					}
				}
				calls.Add(1)
				return metadata.Execute(ctx.Context, arguments)
			},
		}
	}
	validate := func(view *tools.CatalogView) error {
		if !slices.Equal(view.OperationKeys("get_path_info"), []execution.OperationKey{readKey}) {
			return execution.ErrMismatch
		}
		write := view.OperationKeys("write_file")
		if len(write) > 0 && !slices.Equal(write, []execution.OperationKey{writeKey}) {
			return execution.ErrMismatch
		}
		return nil
	}
	master, err := tools.NewCatalog(bind, validate,
		register("inspect", tools.NewGetPathInfoTool(syntheticStatFilesystem{}), trusted, readKey, optionalKey),
		register("change", tools.NewWriteFileTool(syntheticStatFilesystem{}), trusted, writeKey),
		register("inspect", tools.NewReadFileTool(nil, 1024), unsupported, readKey))
	if err != nil {
		t.Fatal(err)
	}
	reader, writer := request("fixture-reader"), request("fixture-writer")
	if err := master.Activate(reader, writer); err != nil {
		t.Fatal(err)
	}
	late, err := execution.BindCatalogFixtureSession(owner, "fixture-late", execution.InheritedSTDIO, []execution.GroupID{"fixture-group"})
	if err != nil {
		t.Fatal(err)
	}
	existing := subjects["fixture-reader"]
	late, err = late.WithPolicy(existing.policy)
	if err != nil {
		t.Fatal(err)
	}
	existing.session = late
	subjects["fixture-late"] = existing
	wrong, err := late.Request(execution.MCP20251125, 4096)
	if err != nil {
		t.Fatal(err)
	}
	writerDispatcher := subjects["fixture-writer"].dispatcher
	if _, err := writerDispatcher.CatalogContext(wrong); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("cross-principal policy was accepted by the catalog: %v", err)
	}
	if _, err := writerDispatcher.Dispatch(t.Context(), wrong, writeKey, nil); !errors.Is(err, execution.ErrMismatch) {
		t.Fatalf("cross-principal policy reached dispatch: %v", err)
	}
	list, call := tools.NewListHandler(master), tools.NewCallHandler(master)
	for _, params := range []string{
		`{"name":"get_path_info","arguments":{"root_id":"work","path":"fixture"}}`,
		`{"name":"write_file","arguments":{"root_id":"work","path":"fixture","content":"x"}}`,
	} {
		if _, err := call.Handle(writer, json.RawMessage(params)); err != nil {
			t.Fatalf("direct call before discovery failed: %v", err)
		}
	}
	for _, args := range []string{`{"wrong":true}`, `{"unbound":true}`, `{"optional":true}`} {
		if _, err := call.Handle(writer, json.RawMessage(`{"name":"get_path_info","arguments":`+args+`}`)); err == nil {
			t.Fatal("a mismatched or unsupported domain route was accepted")
		}
	}
	if wrongEffects.Load() != 0 || wrongResolutions.Load() != 0 || readEffects.Load() != 1 || writeEffects.Load() != 1 {
		t.Fatal("a denied catalog invocation crossed the resolution or effect boundary")
	}
	var requests sync.WaitGroup
	for _, fixture := range []struct {
		context handlers.Context
		names   []string
	}{
		{reader, []string{"get_path_info"}},
		{writer, []string{"get_path_info", "write_file"}},
		{request("fixture-late"), []string{"get_path_info"}},
	} {
		requests.Go(func() {
			for range 30 {
				value, rpcErr := list.Handle(fixture.context, nil)
				if rpcErr != nil {
					t.Error(rpcErr)
					return
				}
				wire, err := json.Marshal(value)
				if err != nil {
					t.Error(err)
					return
				}
				var result struct{ Tools []protocol.Tool }
				if err := json.Unmarshal(wire, &result); err != nil {
					t.Error(err)
					return
				}
				var names []string
				for _, definition := range result.Tools {
					names = append(names, definition.Name)
				}
				if !slices.Equal(names, fixture.names) {
					t.Errorf("wrong effective tools: %v, wanted %v", names, fixture.names)
					return
				}
			}
		})
	}
	requests.Wait()
	missing := handlers.Context{ProtocolVersion: string(execution.MCP20251125)}
	if _, err := list.Handle(missing, nil); err == nil || err.Code != protocol.ErrInvalidParams {
		t.Fatalf("catalog accepted a missing request context: %v", err)
	}
	for _, denied := range []struct {
		context handlers.Context
		args    json.RawMessage
	}{
		{reader, json.RawMessage(`{"name":"write_file"}`)},
		{request("fixture-late"), json.RawMessage(`{"name":"write_file"}`)},
		{writer, json.RawMessage(`{"name":"read_file"}`)},
		{request("fixture-unadmitted"), json.RawMessage(`{"name":"get_path_info"}`)},
		{missing, json.RawMessage(`{"name":"get_path_info"}`)},
	} {
		if _, err := call.Handle(denied.context, denied.args); err == nil || err.Code != protocol.ErrInvalidParams {
			t.Fatalf("hidden or unadmitted definition resolved: %v", err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("a hidden or unadmitted call reached a handler")
	}
	if _, err := call.Handle(request("fixture-late"), json.RawMessage(`{"name":"get_path_info","arguments":{"root_id":"work","path":"fixture"}}`)); err != nil || calls.Load() != 3 {
		t.Fatalf("late principal could not use its validated composition: %v", err)
	}
}
