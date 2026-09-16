package managedprocess

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type policyFunc func(context.Context, StartRequest) (Launch, error)

func (policy policyFunc) AuthorizeStart(ctx context.Context, request StartRequest) (Launch, error) {
	return policy(ctx, request)
}

type fakeStartedProcess struct {
	pid     int
	wait    chan struct{}
	waitErr error
}

func (process *fakeStartedProcess) PID() int { return process.pid }
func (process *fakeStartedProcess) Wait() error {
	<-process.wait
	return process.waitErr
}

func TestEngineStartsAuthorizedProcessAndTracksLifecycle(t *testing.T) {
	var policyRequest StartRequest
	engine := NewEngine(policyFunc(func(_ context.Context, request StartRequest) (Launch, error) {
		policyRequest = request
		return validTestLaunch(), nil
	}))
	started := &fakeStartedProcess{pid: 4242, wait: make(chan struct{})}
	var launched Launch
	engine.start = func(launch Launch) (startedProcess, error) {
		launched = launch
		return started, nil
	}

	request := StartRequest{Principal: "connection-a", Command: "approved", Arguments: []string{"value"}}
	handle, err := engine.Start(context.Background(), request)
	if err != nil || handle == "" {
		t.Fatalf("Start() = (%q, %v), want handle and nil", handle, err)
	}
	if policyRequest.Command != request.Command || launched.Executable != validTestLaunch().Executable {
		t.Fatalf("policy/start mismatch: request=%#v launch=%#v", policyRequest, launched)
	}
	process, ok := engine.Get(request.Principal, handle)
	if !ok || process.PID() != 4242 || process.Status() != StatusRunning {
		t.Fatalf("process = (%v, %t), pid=%d status=%q", process, ok, process.PID(), process.Status())
	}
	if _, ok := engine.Get("connection-b", handle); ok {
		t.Fatal("different principal could get started process")
	}
	close(started.wait)
	waitForStatus(t, process, StatusExited)
}

func TestEngineFailsClosedBeforeStarting(t *testing.T) {
	tests := []struct {
		name    string
		engine  *Engine
		request StartRequest
		wantErr error
	}{
		{"missing principal", NewEngine(policyFunc(allowTestLaunch)), StartRequest{Command: "approved"}, ErrInvalidStartRequest},
		{"missing command", NewEngine(policyFunc(allowTestLaunch)), StartRequest{Principal: "owner"}, ErrInvalidStartRequest},
		{"missing policy", NewEngine(nil), StartRequest{Principal: "owner", Command: "approved"}, ErrStartDenied},
		{"denied", NewEngine(policyFunc(func(context.Context, StartRequest) (Launch, error) { return Launch{}, errors.New("no capability") })), StartRequest{Principal: "owner", Command: "approved"}, ErrStartDenied},
		{"relative executable", NewEngine(policyFunc(func(context.Context, StartRequest) (Launch, error) { return Launch{Executable: "program"}, nil })), StartRequest{Principal: "owner", Command: "approved"}, ErrInvalidLaunch},
		{"relative directory", NewEngine(policyFunc(func(context.Context, StartRequest) (Launch, error) {
			launch := validTestLaunch()
			launch.WorkingDirectory = "relative"
			return launch, nil
		})), StartRequest{Principal: "owner", Command: "approved"}, ErrInvalidLaunch},
		{"malformed environment", NewEngine(policyFunc(func(context.Context, StartRequest) (Launch, error) {
			launch := validTestLaunch()
			launch.Environment = []string{"MISSING_VALUE"}
			return launch, nil
		})), StartRequest{Principal: "owner", Command: "approved"}, ErrInvalidLaunch},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			test.engine.start = func(Launch) (startedProcess, error) {
				called = true
				return nil, errors.New("unexpected")
			}
			handle, err := test.engine.Start(context.Background(), test.request)
			if handle != "" || !errors.Is(err, test.wantErr) || called || test.engine.registry.Len() != 0 {
				t.Fatalf("Start() = (%q, %v), called=%t registered=%d", handle, err, called, test.engine.registry.Len())
			}
		})
	}
}

func TestEngineRetainsFailedStartup(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	engine.start = func(Launch) (startedProcess, error) { return nil, errors.New("rejected") }
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if handle == "" || err == nil {
		t.Fatalf("Start() = (%q, %v), want handle and error", handle, err)
	}
	process, ok := engine.Get("owner", handle)
	if !ok || process.Status() != StatusFailed || process.PID() != 0 {
		t.Fatalf("failed process = (%v, %t), status=%q pid=%d", process, ok, process.Status(), process.PID())
	}
}

func TestEngineRecordsFailedExit(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 7, wait: make(chan struct{}), waitErr: errors.New("exit status 1")}
	engine.start = func(Launch) (startedProcess, error) { return started, nil }
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	process, _ := engine.Get("owner", handle)
	close(started.wait)
	waitForStatus(t, process, StatusFailed)
}

func TestEngineWaitReturnsTerminalResult(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 73, wait: make(chan struct{})}
	engine.start = func(Launch) (startedProcess, error) { return started, nil }
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	resultChannel := make(chan WaitResult, 1)
	errorChannel := make(chan error, 1)
	go func() {
		result, waitErr := engine.Wait(context.Background(), "owner", handle, time.Second)
		resultChannel <- result
		errorChannel <- waitErr
	}()
	select {
	case <-resultChannel:
		t.Fatal("Wait() returned before process became terminal")
	case <-time.After(10 * time.Millisecond):
	}
	close(started.wait)

	result := <-resultChannel
	if waitErr := <-errorChannel; waitErr != nil || result.Status != StatusExited || result.PID != 73 {
		t.Fatalf("Wait() = (%#v, %v), want exited result", result, waitErr)
	}
	// Waiting for an already-terminal process is immediate and stable.
	result, err = engine.Wait(context.Background(), "owner", handle, time.Second)
	if err != nil || result.Status != StatusExited || result.PID != 73 {
		t.Fatalf("second Wait() = (%#v, %v), want same exited result", result, err)
	}
}

func TestEngineWaitTimeoutAndCancellationDoNotChangeProcess(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 81, wait: make(chan struct{})}
	engine.start = func(Launch) (startedProcess, error) { return started, nil }
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	process, _ := engine.Get("owner", handle)

	if result, waitErr := engine.Wait(context.Background(), "owner", handle, time.Millisecond); !errors.Is(waitErr, context.DeadlineExceeded) || result != (WaitResult{}) {
		t.Fatalf("timed Wait() = (%#v, %v), want empty result and deadline", result, waitErr)
	}
	if process.Status() != StatusRunning {
		t.Fatalf("status after wait timeout = %q, want running", process.Status())
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, waitErr := engine.Wait(cancelled, "owner", handle, 0); !errors.Is(waitErr, context.Canceled) || result != (WaitResult{}) {
		t.Fatalf("cancelled Wait() = (%#v, %v), want empty result and cancellation", result, waitErr)
	}
	if process.Status() != StatusRunning {
		t.Fatalf("status after cancelled wait = %q, want running", process.Status())
	}
	close(started.wait)
}

func TestEngineWaitValidatesIdentityAndInput(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 91, wait: make(chan struct{})}
	engine.start = func(Launch) (startedProcess, error) { return started, nil }
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer close(started.wait)

	invalid := []struct {
		name      string
		ctx       context.Context
		principal PrincipalID
		handle    Handle
		timeout   time.Duration
	}{
		{"nil context", nil, "owner", handle, 0},
		{"missing principal", context.Background(), "", handle, 0},
		{"missing handle", context.Background(), "owner", "", 0},
		{"negative timeout", context.Background(), "owner", handle, -time.Second},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, waitErr := engine.Wait(test.ctx, test.principal, test.handle, test.timeout); !errors.Is(waitErr, ErrInvalidWaitRequest) {
				t.Fatalf("Wait() error = %v, want %v", waitErr, ErrInvalidWaitRequest)
			}
		})
	}
	for _, test := range []struct {
		name      string
		principal PrincipalID
		handle    Handle
	}{
		{"wrong owner", "other", handle},
		{"unknown handle", "owner", "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, waitErr := engine.Wait(context.Background(), test.principal, test.handle, 0); !errors.Is(waitErr, ErrProcessUnavailable) {
				t.Fatalf("Wait() error = %v, want %v", waitErr, ErrProcessUnavailable)
			}
		})
	}
}

func TestEngineStartsAndReapsOSProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	engine := NewEngine(policyFunc(func(context.Context, StartRequest) (Launch, error) {
		return Launch{
			Executable:  executable,
			Arguments:   []string{"-test.run=TestManagedProcessHelper", "--"},
			Environment: []string{"FLASHGATE_MANAGED_PROCESS_HELPER=1"},
		}, nil
	}))
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "helper"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	process, ok := engine.Get("owner", handle)
	if !ok || process.PID() <= 0 {
		t.Fatalf("started process = (%v, %t), pid=%d", process, ok, process.PID())
	}
	waitForStatus(t, process, StatusExited)
}

func TestManagedProcessHelper(t *testing.T) {
	if os.Getenv("FLASHGATE_MANAGED_PROCESS_HELPER") != "1" {
		return
	}
	os.Exit(0)
}

func allowTestLaunch(context.Context, StartRequest) (Launch, error) { return validTestLaunch(), nil }

func validTestLaunch() Launch {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = `C:\\`
	}
	return Launch{Executable: filepath.Join(root, "approved", "program"), Arguments: []string{"arg"}, WorkingDirectory: root, Environment: []string{"SAFE=value"}}
}

func waitForStatus(t *testing.T, process *Process, want Status) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if process.Status() == want {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("status = %q, want %q", process.Status(), want)
}
