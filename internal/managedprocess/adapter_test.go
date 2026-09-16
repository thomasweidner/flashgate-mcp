package managedprocess

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestEngineRejectsInvalidResourceLimitsBeforeAdapter(t *testing.T) {
	called := false
	adapter := processAdapterFunc(func(Launch, io.Writer, io.Writer) (startedProcess, error) {
		called = true
		return nil, errors.New("unexpected")
	})
	engine, err := NewEngineWithAdapterConfiguration(
		policyFunc(func(context.Context, StartRequest) (Launch, error) {
			launch := validTestLaunch()
			launch.Resources.CPURate = MaxCPURate + 1
			return launch, nil
		}),
		DefaultLimits(), DefaultRuntimeLimits(), adapter,
	)
	if err != nil {
		t.Fatalf("NewEngineWithAdapterConfiguration() error = %v", err)
	}
	if handle, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"}); handle != "" || !errors.Is(startErr, ErrInvalidResourceLimits) || called {
		t.Fatalf("Start() = (%q, %v), adapter called=%t", handle, startErr, called)
	}
}

func TestEnginePassesPolicyResourceLimitsOnlyToAdapter(t *testing.T) {
	want := ResourceLimits{CPURate: 1250, MemoryBytes: 64 << 20}
	var got ResourceLimits
	adapter := processAdapterFunc(func(launch Launch, _ io.Writer, _ io.Writer) (startedProcess, error) {
		got = launch.Resources
		return nil, errors.New("synthetic start failure")
	})
	engine, err := NewEngineWithAdapterConfiguration(
		policyFunc(func(context.Context, StartRequest) (Launch, error) {
			launch := validTestLaunch()
			launch.Resources = want
			return launch, nil
		}),
		DefaultLimits(), DefaultRuntimeLimits(), adapter,
	)
	if err != nil {
		t.Fatalf("NewEngineWithAdapterConfiguration() error = %v", err)
	}
	handle, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if handle == "" || !errors.Is(startErr, ErrProcessStartFailed) || got != want {
		t.Fatalf("Start() = (%q, %v), adapter limits=%#v, want %#v", handle, startErr, got, want)
	}
}

func TestEngineRejectsNilAdapter(t *testing.T) {
	engine, err := NewEngineWithAdapterConfiguration(policyFunc(allowTestLaunch), DefaultLimits(), DefaultRuntimeLimits(), nil)
	if engine != nil || !errors.Is(err, ErrInvalidProcessAdapter) {
		t.Fatalf("NewEngineWithAdapterConfiguration(nil) = (%v, %v)", engine, err)
	}
}
