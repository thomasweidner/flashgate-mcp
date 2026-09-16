package managedprocess

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
)

func TestEngineEnforcesGlobalAndProfileProcessLimits(t *testing.T) {
	engine, err := NewEngineWithLimits(policyFunc(func(_ context.Context, request StartRequest) (Launch, error) {
		launch := validTestLaunch()
		launch.Profile = request.Command
		return launch, nil
	}), Limits{Global: 2, PerProfile: 1})
	if err != nil {
		t.Fatalf("NewEngineWithLimits() error = %v", err)
	}

	var mu sync.Mutex
	started := make([]*fakeStartedProcess, 0, 2)
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) {
		mu.Lock()
		defer mu.Unlock()
		process := &fakeStartedProcess{pid: len(started) + 1, wait: make(chan struct{})}
		started = append(started, process)
		return process, nil
	}

	start := func(profile string) error {
		_, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: profile})
		return startErr
	}
	if err := start("profile-a"); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if err := start("profile-a"); !errors.Is(err, ErrProcessLimitReached) {
		t.Fatalf("same-profile Start() error = %v, want %v", err, ErrProcessLimitReached)
	}
	if err := start("profile-b"); err != nil {
		t.Fatalf("second-profile Start() error = %v", err)
	}
	if err := start("profile-c"); !errors.Is(err, ErrProcessLimitReached) {
		t.Fatalf("global-limit Start() error = %v, want %v", err, ErrProcessLimitReached)
	}
	if len(started) != 2 {
		t.Fatalf("started process count = %d, want 2", len(started))
	}

	close(started[0].wait)
	waitForStatus(t, mustGetProcess(t, engine, "owner", 0), StatusExited)
	if err := start("profile-a"); err != nil {
		t.Fatalf("Start() after terminal release error = %v", err)
	}
	close(started[1].wait)
	close(started[2].wait)
}

func TestEngineRejectsInvalidProcessLimits(t *testing.T) {
	for _, limits := range []Limits{{}, {Global: 1}, {PerProfile: 1}, {Global: 1, PerProfile: 2}} {
		if engine, err := NewEngineWithLimits(policyFunc(allowTestLaunch), limits); engine != nil || !errors.Is(err, ErrInvalidProcessLimits) {
			t.Fatalf("NewEngineWithLimits(%#v) = (%v, %v), want nil and %v", limits, engine, err, ErrInvalidProcessLimits)
		}
	}
}

func TestEngineLimitAdmissionIsAtomic(t *testing.T) {
	engine, err := NewEngineWithLimits(policyFunc(allowTestLaunch), Limits{Global: 3, PerProfile: 3})
	if err != nil {
		t.Fatalf("NewEngineWithLimits() error = %v", err)
	}

	var mu sync.Mutex
	started := make([]*fakeStartedProcess, 0, 3)
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) {
		mu.Lock()
		defer mu.Unlock()
		process := &fakeStartedProcess{pid: len(started) + 1, wait: make(chan struct{})}
		started = append(started, process)
		return process, nil
	}

	const attempts = 24
	results := make(chan error, attempts)
	var callers sync.WaitGroup
	for range attempts {
		callers.Add(1)
		go func() {
			defer callers.Done()
			_, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
			results <- startErr
		}()
	}
	callers.Wait()
	close(results)

	accepted := 0
	for startErr := range results {
		switch {
		case startErr == nil:
			accepted++
		case errors.Is(startErr, ErrProcessLimitReached):
		default:
			t.Fatalf("Start() error = %v", startErr)
		}
	}
	if accepted != 3 || len(started) != 3 {
		t.Fatalf("accepted/started = %d/%d, want 3/3", accepted, len(started))
	}
	for _, process := range started {
		close(process.wait)
	}
}

func mustGetProcess(t *testing.T, engine *Engine, principal PrincipalID, index int) *Process {
	t.Helper()
	engine.registry.mu.RLock()
	defer engine.registry.mu.RUnlock()
	for _, registered := range engine.registry.entries {
		if registered.principal == principal && registered.process.PID() == index+1 {
			return registered.process
		}
	}
	t.Fatalf("process with index %d not found", index)
	return nil
}
