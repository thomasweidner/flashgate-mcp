package executionidentity

import (
	"errors"
	"testing"
	"time"
)

func stateContext(t *testing.T) ExecutionContext {
	t.Helper()
	caller, err := NewCaller("principal-1", []string{"readers"})
	if err != nil {
		t.Fatal(err)
	}
	return ExecutionContext{caller: caller, profile: "safe-read", rootID: "docs", backend: BackendServiceAccount, serviceInstance: "service-2", serviceGeneration: "generation-7", protocolContext: "mcp-session-4", expiresAt: time.Now().Add(time.Hour)}
}

func TestBoundStoreRequiresCompleteExecutionContext(t *testing.T) {
	owner := stateContext(t)
	store := NewBoundStore[string]()
	if err := store.Put("opaque-handle", owner, "result"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get("opaque-handle", owner); err != nil || got != "result" {
		t.Fatalf("Get() = %q, %v", got, err)
	}

	tests := []struct {
		name   string
		mutate func(*ExecutionContext)
	}{
		{"principal", func(c *ExecutionContext) { c.caller, _ = NewCaller("principal-2", []string{"readers"}) }},
		{"groups", func(c *ExecutionContext) { c.caller, _ = NewCaller("principal-1", []string{"writers"}) }},
		{"profile", func(c *ExecutionContext) { c.profile = "write" }},
		{"root", func(c *ExecutionContext) { c.rootID = "other" }},
		{"backend", func(c *ExecutionContext) { c.backend = BackendCurrentProcess }},
		{"instance", func(c *ExecutionContext) { c.serviceInstance = "service-3" }},
		{"generation", func(c *ExecutionContext) { c.serviceGeneration = "generation-8" }},
		{"protocol", func(c *ExecutionContext) { c.protocolContext = "mcp-session-5" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			other := owner
			test.mutate(&other)
			if _, err := store.Get("opaque-handle", other); !errors.Is(err, ErrStateAccessDenied) {
				t.Fatalf("Get() error = %v", err)
			}
		})
	}
}

func TestBoundStoreExpiryDeletionAndNoTakeover(t *testing.T) {
	owner := stateContext(t)
	store := NewBoundStore[int]()
	now := time.Now()
	store.now = func() time.Time { return now }
	owner.expiresAt = now.Add(time.Minute)
	if err := store.Put("job", owner, 42); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("job", owner, 99); !errors.Is(err, ErrStateAccessDenied) {
		t.Fatalf("replacement error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := store.Get("job", owner); !errors.Is(err, ErrStateExpired) {
		t.Fatalf("expiry error = %v", err)
	}
	if _, err := store.Get("job", owner); !errors.Is(err, ErrStateAccessDenied) {
		t.Fatalf("removed entry error = %v", err)
	}

	fresh := stateContext(t)
	store.now = time.Now
	if err := store.Put("cancel", fresh, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("cancel", fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("cancel", fresh); !errors.Is(err, ErrStateAccessDenied) {
		t.Fatalf("deleted entry error = %v", err)
	}
}
