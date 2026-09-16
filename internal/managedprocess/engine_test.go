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
