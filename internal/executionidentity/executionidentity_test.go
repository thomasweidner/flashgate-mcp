package executionidentity

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type operation string

func (o operation) Name() string { return string(o) }

type policyFunc func(context.Context, Caller, Request) (Decision, error)

func (f policyFunc) Authorize(ctx context.Context, c Caller, r Request) (Decision, error) {
	return f(ctx, c, r)
}

type adapterFunc func(context.Context, ExecutionContext, Operation) (any, error)

func (f adapterFunc) Execute(ctx context.Context, e ExecutionContext, o Operation) (any, error) {
	return f(ctx, e, o)
}

func approvedDecision(backend BackendID) Decision {
	return Decision{Allowed: true, Profile: "safe-read", RootID: "docs", Capability: "filesystem.read", Backend: backend, ServiceGeneration: "generation-7"}
}

func validRequest() Request {
	return Request{RootID: "docs", Capability: "filesystem.read", Operation: "stat", Correlation: "request-9"}
}

func TestDispatchBindsCallerPolicyAndBackend(t *testing.T) {
	groups := []string{"writers", "readers"}
	caller, err := NewCaller("principal-1", groups)
	if err != nil {
		t.Fatal(err)
	}
	groups[0] = "mutated"

	backend, err := NewPassthroughBackend(BackendServiceAccount)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(backend)
	if err != nil {
		t.Fatal(err)
	}

	var got ExecutionContext
	dispatcher, err := NewDispatcher(
		policyFunc(func(_ context.Context, c Caller, r Request) (Decision, error) {
			if c.Principal() != "principal-1" || r.RootID != "docs" {
				t.Fatal("policy received wrong identity")
			}
			return approvedDecision(BackendServiceAccount), nil
		}),
		registry,
		adapterFunc(func(_ context.Context, binding ExecutionContext, op Operation) (any, error) {
			got = binding
			return op.Name(), nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := dispatcher.Dispatch(context.Background(), caller, validRequest(), operation("stat"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "stat" {
		t.Fatalf("result = %v", result)
	}
	if got.Caller().Principal() != "principal-1" || !reflect.DeepEqual(got.Caller().Groups(), []string{"readers", "writers"}) {
		t.Fatalf("caller binding = %q %v", got.Caller().Principal(), got.Caller().Groups())
	}
	if got.Profile() != "safe-read" || got.RootID() != "docs" || got.Capability() != "filesystem.read" || got.Backend() != BackendServiceAccount || got.ServiceGeneration() != "generation-7" || got.Correlation() != "request-9" {
		t.Fatalf("incomplete execution binding: %+v", got)
	}
	copy := got.Caller().Groups()
	copy[0] = "changed"
	if got.Caller().Groups()[0] != "readers" {
		t.Fatal("caller groups were mutable")
	}
}

func TestDispatchFailsClosedBeforeBackend(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		want     error
	}{
		{"denied", Decision{}, ErrDenied},
		{"root mismatch", func() Decision { d := approvedDecision(BackendServiceAccount); d.RootID = "other"; return d }(), ErrInvalidContext},
		{"capability mismatch", func() Decision {
			d := approvedDecision(BackendServiceAccount)
			d.Capability = "filesystem.write"
			return d
		}(), ErrInvalidContext},
		{"user worker reserved", approvedDecision(BackendUserWorker), ErrBackendUnavailable},
		{"unknown backend", approvedDecision("unknown"), ErrBackendUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend, _ := NewPassthroughBackend(BackendServiceAccount)
			registry, _ := NewRegistry(backend)
			called := false
			dispatcher, _ := NewDispatcher(policyFunc(func(context.Context, Caller, Request) (Decision, error) { return test.decision, nil }), registry, adapterFunc(func(context.Context, ExecutionContext, Operation) (any, error) { called = true; return nil, nil }))
			caller, _ := NewCaller("principal", nil)
			_, err := dispatcher.Dispatch(context.Background(), caller, validRequest(), operation("stat"))
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if called {
				t.Fatal("denied operation reached OS adapter")
			}
		})
	}
}

func TestDispatchPreservesContextCancellation(t *testing.T) {
	backend, _ := NewPassthroughBackend(BackendCurrentProcess)
	registry, _ := NewRegistry(backend)
	dispatcher, _ := NewDispatcher(policyFunc(func(context.Context, Caller, Request) (Decision, error) {
		return approvedDecision(BackendCurrentProcess), nil
	}), registry, adapterFunc(func(ctx context.Context, _ ExecutionContext, _ Operation) (any, error) { return nil, ctx.Err() }))
	caller, _ := NewCaller("principal", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := dispatcher.Dispatch(ctx, caller, validRequest(), operation("stat"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestRegistryAndConstructionValidation(t *testing.T) {
	current, _ := NewPassthroughBackend(BackendCurrentProcess)
	if _, err := NewRegistry(current, current); !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := NewPassthroughBackend(BackendUserWorker); !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("user-worker error = %v", err)
	}
	if _, err := NewCaller("", nil); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("caller error = %v", err)
	}
	if _, err := NewCaller("principal", []string{"same", "same"}); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("groups error = %v", err)
	}
}
