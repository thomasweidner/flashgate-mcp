package executionidentity

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"
)

type auditRecorder struct{ events []AuditEvent }

func (r *auditRecorder) Record(_ context.Context, event AuditEvent) {
	r.events = append(r.events, event)
}

func serviceBinding(t *testing.T, root string) ExecutionContext {
	t.Helper()
	caller, err := NewCaller("caller-42", []string{"developers"})
	if err != nil {
		t.Fatal(err)
	}
	return ExecutionContext{caller: caller, profile: "safe-read", rootID: root, capability: "filesystem.read", backend: BackendServiceAccount, serviceGeneration: "generation-2", correlation: "request-8"}
}

func TestServiceAccountBackendDispatchAndDualIdentityAudit(t *testing.T) {
	audit := &auditRecorder{}
	backend, err := NewServiceAccountBackend(EffectiveIdentity{Principal: "flashgate-service"}, []string{"shared"}, audit)
	if err != nil {
		t.Fatal(err)
	}
	result, err := backend.Dispatch(context.Background(), serviceBinding(t, "shared"), operation("stat"), adapterFunc(func(_ context.Context, _ ExecutionContext, _ Operation) (any, error) { return "ok", nil }))
	if err != nil || result != "ok" {
		t.Fatalf("result, error = %v, %v", result, err)
	}
	want := AuditEvent{CallerPrincipal: "caller-42", EffectivePrincipal: "flashgate-service", Backend: BackendServiceAccount, RootID: "shared", Capability: "filesystem.read", Operation: "stat", Correlation: "request-8", ServiceGeneration: "generation-2", Result: "allowed"}
	if !reflect.DeepEqual(audit.events, []AuditEvent{want}) {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestServiceAccountBackendDeniesUngrantedRootBeforeAdapter(t *testing.T) {
	audit := &auditRecorder{}
	backend, _ := NewServiceAccountBackend(EffectiveIdentity{Principal: "flashgate-service"}, []string{"shared"}, audit)
	called := false
	_, err := backend.Dispatch(context.Background(), serviceBinding(t, "personal"), operation("stat"), adapterFunc(func(context.Context, ExecutionContext, Operation) (any, error) { called = true; return nil, nil }))
	if !errors.Is(err, ErrDenied) || called {
		t.Fatalf("error, adapter called = %v, %v", err, called)
	}
	if len(audit.events) != 1 || audit.events[0].Result != "denied_root" {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestServiceAccountBackendNormalizesOSPermissionDenial(t *testing.T) {
	audit := &auditRecorder{}
	backend, _ := NewServiceAccountBackend(EffectiveIdentity{Principal: "flashgate-service"}, []string{"shared"}, audit)
	_, err := backend.Dispatch(context.Background(), serviceBinding(t, "shared"), operation("read"), adapterFunc(func(context.Context, ExecutionContext, Operation) (any, error) { return nil, fs.ErrPermission }))
	if !errors.Is(err, ErrDenied) || errors.Is(err, fs.ErrPermission) {
		t.Fatalf("error = %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Result != "denied_os_permission" {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestServiceAccountBackendRejectsUnsafeConfiguration(t *testing.T) {
	audit := &auditRecorder{}
	tests := []struct {
		name     string
		identity EffectiveIdentity
		roots    []string
		sink     AuditSink
	}{
		{"empty identity", EffectiveIdentity{}, []string{"shared"}, audit},
		{"privileged identity", EffectiveIdentity{Principal: "root", Privileged: true}, []string{"shared"}, audit},
		{"no roots", EffectiveIdentity{Principal: "flashgate-service"}, nil, audit},
		{"empty root", EffectiveIdentity{Principal: "flashgate-service"}, []string{""}, audit},
		{"duplicate root", EffectiveIdentity{Principal: "flashgate-service"}, []string{"shared", "shared"}, audit},
		{"no audit", EffectiveIdentity{Principal: "flashgate-service"}, []string{"shared"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewServiceAccountBackend(test.identity, test.roots, test.sink); !errors.Is(err, ErrBackendUnavailable) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
