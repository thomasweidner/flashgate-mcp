package managedprocess

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestEnginePIDReuseDoesNotAliasHandles(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	waits := []chan struct{}{make(chan struct{}), make(chan struct{})}
	started := 0
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) {
		process := &fakeStartedProcess{pid: 4242, wait: waits[started]}
		started++
		return process, nil
	}

	first, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	close(waits[0])
	firstProcess, _ := engine.Get("owner", first)
	waitForStatus(t, firstProcess, StatusExited)

	second, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	defer close(waits[1])
	if first == second {
		t.Fatalf("distinct processes with reused PID received the same handle %q", first)
	}
	secondProcess, _ := engine.Get("owner", second)
	if firstProcess.PID() != secondProcess.PID() || firstProcess.Status() != StatusExited || secondProcess.Status() != StatusRunning {
		t.Fatalf("PID reuse aliased lifecycle: first=%#v second=%#v", firstProcess.Evidence(), secondProcess.Evidence())
	}
}

func TestEngineRestartInvalidatesPriorHandles(t *testing.T) {
	beforeRestart := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 5150, wait: make(chan struct{})}
	beforeRestart.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) { return started, nil }
	handle, err := beforeRestart.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer close(started.wait)

	afterRestart := NewEngine(policyFunc(allowTestLaunch))
	if _, ok := afterRestart.Get("owner", handle); ok {
		t.Fatalf("new engine resolved pre-restart handle %q", handle)
	}
	if _, err := afterRestart.Wait(context.Background(), "owner", handle, time.Millisecond); !errors.Is(err, ErrProcessUnavailable) {
		t.Fatalf("new engine Wait() error = %v, want %v", err, ErrProcessUnavailable)
	}
	if _, err := afterRestart.Stop(context.Background(), "owner", handle); !errors.Is(err, ErrProcessUnavailable) {
		t.Fatalf("new engine Stop() error = %v, want %v", err, ErrProcessUnavailable)
	}
}

func TestEngineStopExitRaceHasOneTerminalOutcomeAndReleasesOneSlot(t *testing.T) {
	engine, err := NewEngineWithLimits(policyFunc(allowTestLaunch), Limits{Global: 1, PerProfile: 1})
	if err != nil {
		t.Fatalf("NewEngineWithLimits() error = %v", err)
	}

	first := &fakeStartedProcess{pid: 91, wait: make(chan struct{})}
	second := &fakeStartedProcess{pid: 92, wait: make(chan struct{})}
	var startMu sync.Mutex
	startCount := 0
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) {
		startMu.Lock()
		defer startMu.Unlock()
		startCount++
		if startCount == 1 {
			return first, nil
		}
		return second, nil
	}

	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	process, _ := engine.Get("owner", handle)

	start := make(chan struct{})
	stopResult := make(chan error, 1)
	go func() {
		<-start
		_, stopErr := engine.Stop(context.Background(), "owner", handle)
		stopResult <- stopErr
	}()
	close(start)
	close(first.wait)
	if stopErr := <-stopResult; stopErr != nil {
		t.Fatalf("Stop() error = %v", stopErr)
	}
	waitForTerminalStatus(t, process)
	terminal := process.Status()
	if terminal != StatusExited && terminal != StatusStopped {
		t.Fatalf("race status = %q, want exited or stopped", terminal)
	}
	time.Sleep(time.Millisecond)
	if process.Status() != terminal {
		t.Fatalf("terminal status changed from %q to %q", terminal, process.Status())
	}

	if _, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"}); err != nil {
		t.Fatalf("Start() after raced completion error = %v", err)
	}
	close(second.wait)
}

func TestEngineFailedStartsReleaseConcurrencyLimit(t *testing.T) {
	engine, err := NewEngineWithLimits(policyFunc(allowTestLaunch), Limits{Global: 1, PerProfile: 1})
	if err != nil {
		t.Fatalf("NewEngineWithLimits() error = %v", err)
	}
	engine.start = func(Launch, io.Writer, io.Writer) (startedProcess, error) {
		return nil, errors.New("synthetic adapter failure")
	}

	for attempt := 0; attempt < 8; attempt++ {
		handle, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
		if handle == "" || !errors.Is(startErr, ErrProcessStartFailed) {
			t.Fatalf("attempt %d Start() = (%q, %v), want retained failed handle", attempt, handle, startErr)
		}
		process, ok := engine.Get("owner", handle)
		if !ok || process.Status() != StatusFailed {
			t.Fatalf("attempt %d failed lifecycle = (%v, %t)", attempt, process, ok)
		}
	}
}

func waitForTerminalStatus(t *testing.T, process *Process) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !process.Status().Terminal() {
		if time.Now().After(deadline) {
			t.Fatalf("process remained nonterminal in %q", process.Status())
		}
		time.Sleep(time.Millisecond)
	}
}
