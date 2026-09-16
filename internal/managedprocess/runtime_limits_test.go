package managedprocess

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestEngineEnforcesConfiguredProcessRuntime(t *testing.T) {
	engine, err := NewEngineWithConfiguration(
		policyFunc(func(context.Context, StartRequest) (Launch, error) {
			launch := validTestLaunch()
			launch.Runtime = 10 * time.Millisecond
			return launch, nil
		}),
		Limits{Global: 1, PerProfile: 1},
		RuntimeLimits{Default: time.Second, Maximum: time.Second},
	)
	if err != nil {
		t.Fatalf("NewEngineWithConfiguration() error = %v", err)
	}
	started := &fakeStartedProcess{pid: 201, wait: make(chan struct{})}
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) { return started, nil }

	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	process, _ := engine.Get("owner", handle)
	waitForStatus(t, process, StatusTimedOut)
	if started.killCount() != 1 {
		t.Fatalf("runtime termination count = %d, want 1", started.killCount())
	}
	result, err := engine.Wait(context.Background(), "owner", handle, 0)
	if err != nil || result.Status != StatusTimedOut {
		t.Fatalf("Wait() = (%#v, %v), want timed_out", result, err)
	}

	// Timeout releases the active-process reservation exactly once, even though
	// the test double's Wait remains blocked until cleanup below.
	if _, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"}); err != nil {
		t.Fatalf("Start() after runtime timeout error = %v", err)
	}
	close(started.wait)
}

func TestEngineUsesDefaultRuntimeAndCancelsTimerAfterExit(t *testing.T) {
	engine, err := NewEngineWithConfiguration(
		policyFunc(allowTestLaunch),
		Limits{Global: 1, PerProfile: 1},
		RuntimeLimits{Default: 25 * time.Millisecond, Maximum: time.Second},
	)
	if err != nil {
		t.Fatalf("NewEngineWithConfiguration() error = %v", err)
	}
	started := &fakeStartedProcess{pid: 202, wait: make(chan struct{})}
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) { return started, nil }
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	process, _ := engine.Get("owner", handle)
	close(started.wait)
	waitForStatus(t, process, StatusExited)
	time.Sleep(40 * time.Millisecond)
	if started.killCount() != 0 || process.Status() != StatusExited {
		t.Fatalf("completed process status/kills = %q/%d, want exited/0", process.Status(), started.killCount())
	}
}

func TestEngineRejectsInvalidAndExcessiveRuntimeLimits(t *testing.T) {
	for _, runtimeLimits := range []RuntimeLimits{
		{},
		{Default: time.Second},
		{Maximum: time.Second},
		{Default: 2 * time.Second, Maximum: time.Second},
	} {
		engine, err := NewEngineWithConfiguration(policyFunc(allowTestLaunch), Limits{Global: 1, PerProfile: 1}, runtimeLimits)
		if engine != nil || !errors.Is(err, ErrInvalidRuntimeLimits) {
			t.Fatalf("NewEngineWithConfiguration(%#v) = (%v, %v), want nil and %v", runtimeLimits, engine, err, ErrInvalidRuntimeLimits)
		}
	}

	for _, requested := range []time.Duration{-time.Second, 2 * time.Second} {
		started := false
		engine, err := NewEngineWithConfiguration(
			policyFunc(func(context.Context, StartRequest) (Launch, error) {
				launch := validTestLaunch()
				launch.Runtime = requested
				return launch, nil
			}),
			Limits{Global: 1, PerProfile: 1},
			RuntimeLimits{Default: time.Second, Maximum: time.Second},
		)
		if err != nil {
			t.Fatalf("NewEngineWithConfiguration() error = %v", err)
		}
		engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) {
			started = true
			return nil, errors.New("unexpected start")
		}
		if handle, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"}); handle != "" || !errors.Is(startErr, ErrInvalidLaunch) || started {
			t.Fatalf("Start(runtime=%s) = (%q, %v), started=%t; want pre-launch rejection", requested, handle, startErr, started)
		}
	}
}
