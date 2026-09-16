package managedprocess

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrStartDenied         = errors.New("managed process start denied")
	ErrInvalidStartRequest = errors.New("invalid managed process start request")
	ErrInvalidLaunch       = errors.New("invalid managed process launch")
	ErrInvalidWaitRequest  = errors.New("invalid managed process wait request")
	ErrProcessUnavailable  = errors.New("managed process unavailable")
)

// WaitResult is the immutable final result of waiting for a managed process.
// PID remains diagnostic and must not be used as authority for later actions.
type WaitResult struct {
	Status Status
	PID    int
}

// StartRequest contains untrusted, transport-neutral input for start_process.
// Command identifies a server-side policy entry, not a path or shell command.
type StartRequest struct {
	Principal PrincipalID
	Command   string
	Arguments []string
}

// Launch is the fully authorized process description returned by Policy.
// Environment is complete; the engine never inherits the server environment.
type Launch struct {
	Executable       string
	Arguments        []string
	WorkingDirectory string
	Environment      []string
}

// Policy owns capability, profile, executable, argument, directory,
// environment, and risk-policy checks. Principal must come from trusted state.
type Policy interface {
	AuthorizeStart(context.Context, StartRequest) (Launch, error)
}

// Process is one engine-owned process. PID is diagnostic; its opaque handle is
// the authority for later managed-process operations.
type Process struct {
	state *State
	mu    sync.RWMutex
	pid   int
}

func (process *Process) Status() Status { return process.state.Status() }

func (process *Process) PID() int {
	process.mu.RLock()
	defer process.mu.RUnlock()
	return process.pid
}

func (process *Process) setPID(pid int) {
	process.mu.Lock()
	defer process.mu.Unlock()
	process.pid = pid
}

// Engine starts only policy-authorized processes and records them in the
// principal-bound managed registry.
type Engine struct {
	policy   Policy
	registry *Registry[*Process]
	start    func(Launch) (startedProcess, error)
}

type startedProcess interface {
	PID() int
	Wait() error
}

// NewEngine creates an engine. A nil policy is fail-closed.
func NewEngine(policy Policy) *Engine {
	return &Engine{policy: policy, registry: NewRegistry[*Process](), start: startOSProcess}
}

// Start authorizes, starts, and registers one process. When OS startup fails
// after registration, the returned handle remains valid in terminal Failed
// state, preserving an unambiguous managed lifecycle record.
func (engine *Engine) Start(ctx context.Context, request StartRequest) (Handle, error) {
	if request.Principal == "" || request.Command == "" {
		return "", ErrInvalidStartRequest
	}
	if engine.policy == nil {
		return "", ErrStartDenied
	}

	launch, err := engine.policy.AuthorizeStart(ctx, cloneRequest(request))
	if err != nil {
		return "", ErrStartDenied
	}
	if err := validateLaunch(launch); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrStartDenied, err)
	}

	process := &Process{state: NewState()}
	handle, err := engine.registry.Register(request.Principal, process)
	if err != nil {
		return "", err
	}

	started, err := engine.start(cloneLaunch(launch))
	if err != nil {
		_ = process.state.Transition(StatusFailed)
		return handle, fmt.Errorf("start process: %w", err)
	}
	if started.PID() <= 0 {
		_ = process.state.Transition(StatusFailed)
		go func() { _ = started.Wait() }()
		return handle, fmt.Errorf("start process: %w: missing process identifier", ErrInvalidLaunch)
	}
	process.setPID(started.PID())
	if err := process.state.Transition(StatusRunning); err != nil {
		return handle, fmt.Errorf("record running process: %w", err)
	}

	go reapProcess(process, started)
	return handle, nil
}

// Get returns a process only to the principal that started it.
func (engine *Engine) Get(principal PrincipalID, handle Handle) (*Process, bool) {
	return engine.registry.Get(principal, handle)
}

// Wait returns only after the principal-owned process reaches a terminal
// state. timeout bounds this wait when positive; zero relies on ctx alone.
// A wait timeout or cancellation does not stop the process or change its
// lifecycle status. Unknown and wrong-principal handles are indistinguishable.
func (engine *Engine) Wait(ctx context.Context, principal PrincipalID, handle Handle, timeout time.Duration) (WaitResult, error) {
	if ctx == nil || principal == "" || handle == "" || timeout < 0 {
		return WaitResult{}, ErrInvalidWaitRequest
	}
	process, ok := engine.registry.Get(principal, handle)
	if !ok {
		return WaitResult{}, ErrProcessUnavailable
	}

	waitContext := ctx
	cancel := func() {}
	if timeout > 0 {
		waitContext, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	status, err := process.state.Wait(waitContext)
	if err != nil {
		return WaitResult{}, fmt.Errorf("wait process: %w", err)
	}
	return WaitResult{Status: status, PID: process.PID()}, nil
}

func cloneRequest(request StartRequest) StartRequest {
	request.Arguments = append([]string(nil), request.Arguments...)
	return request
}

func cloneLaunch(launch Launch) Launch {
	launch.Arguments = append([]string(nil), launch.Arguments...)
	launch.Environment = append([]string(nil), launch.Environment...)
	return launch
}

func validateLaunch(launch Launch) error {
	if launch.Executable == "" || !filepath.IsAbs(launch.Executable) {
		return fmt.Errorf("%w: executable must be an absolute path", ErrInvalidLaunch)
	}
	if launch.WorkingDirectory != "" && !filepath.IsAbs(launch.WorkingDirectory) {
		return fmt.Errorf("%w: working directory must be an absolute path", ErrInvalidLaunch)
	}
	for _, entry := range launch.Environment {
		separator := strings.IndexByte(entry, '=')
		if separator <= 0 || strings.ContainsRune(entry, '\x00') {
			return fmt.Errorf("%w: malformed environment entry", ErrInvalidLaunch)
		}
	}
	return nil
}

func startOSProcess(launch Launch) (startedProcess, error) {
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Dir = launch.WorkingDirectory
	command.Env = append([]string(nil), launch.Environment...)
	if err := command.Start(); err != nil {
		return nil, err
	}
	return osProcess{command}, nil
}

type osProcess struct{ command *exec.Cmd }

func (process osProcess) PID() int    { return process.command.Process.Pid }
func (process osProcess) Wait() error { return process.command.Wait() }

func reapProcess(process *Process, started startedProcess) {
	next := StatusExited
	if err := started.Wait(); err != nil {
		next = StatusFailed
	}
	// A future stop/timeout path may win. State preserves the first terminal
	// outcome, so reaping cannot overwrite it.
	_ = process.state.Transition(next)
}
