package managedprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	ErrInvalidReadRequest  = errors.New("invalid managed process output read request")
	ErrInvalidStopRequest  = errors.New("invalid managed process stop request")
	ErrProcessUnavailable  = errors.New("managed process unavailable")
	ErrStopFailed          = errors.New("managed process stop failed")
)

// WaitResult is the immutable final result of waiting for a managed process.
// PID remains diagnostic and must not be used as authority for later actions.
type WaitResult struct {
	Status Status
	PID    int
}

// StopResult describes the stable state after a managed stop request. PID is
// diagnostic only; the principal-bound handle remains the control authority.
type StopResult struct {
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
	// Profile is a trusted policy-selected budget scope. An empty value uses
	// the default profile and never a caller-provided identity.
	Profile string
	// Runtime is the policy-authorized maximum lifetime for this process. Zero
	// selects the engine default; negative values and values above the engine
	// maximum fail closed before operating-system launch.
	Runtime time.Duration
}

// Policy owns capability, profile, executable, argument, directory,
// environment, and risk-policy checks. Principal must come from trusted state.
type Policy interface {
	AuthorizeStart(context.Context, StartRequest) (Launch, error)
}

// Process is one engine-owned process. PID is diagnostic; its opaque handle is
// the authority for later managed-process operations.
type Process struct {
	state        *State
	output       *outputBuffer
	mu           sync.RWMutex
	control      sync.Mutex
	release      sync.Once
	releaseLimit func()
	started      startedProcess
	pid          int
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

func (process *Process) setStarted(started startedProcess) {
	process.mu.Lock()
	defer process.mu.Unlock()
	process.started = started
}

func (process *Process) startedProcess() startedProcess {
	process.mu.RLock()
	defer process.mu.RUnlock()
	return process.started
}

func (process *Process) releaseBudget() {
	process.release.Do(process.releaseLimit)
}

// Engine starts only policy-authorized processes and records them in the
// principal-bound managed registry.
type Engine struct {
	policy   Policy
	registry *Registry[*Process]
	limiter  *limiter
	runtime  RuntimeLimits
	start    func(Launch, io.Writer, io.Writer) (startedProcess, error)
}

type startedProcess interface {
	PID() int
	Wait() error
	Kill() error
}

// NewEngine creates an engine. A nil policy is fail-closed.
func NewEngine(policy Policy) *Engine {
	engine, err := NewEngineWithLimits(policy, DefaultLimits())
	if err != nil {
		panic(err)
	}
	return engine
}

// NewEngineWithLimits creates an engine with explicit global and per-profile
// active-process budgets. Invalid or disabling budgets fail closed.
func NewEngineWithLimits(policy Policy, limits Limits) (*Engine, error) {
	return NewEngineWithConfiguration(policy, limits, DefaultRuntimeLimits())
}

// NewEngineWithConfiguration creates an engine with explicit concurrency and
// runtime boundaries. Invalid or disabling limits fail closed.
func NewEngineWithConfiguration(policy Policy, limits Limits, runtimeLimits RuntimeLimits) (*Engine, error) {
	processLimiter, err := newLimiter(limits)
	if err != nil {
		return nil, err
	}
	if err := runtimeLimits.validate(); err != nil {
		return nil, err
	}
	return &Engine{policy: policy, registry: NewRegistry[*Process](), limiter: processLimiter, runtime: runtimeLimits, start: startOSProcess}, nil
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
	runtimeLimit, err := engine.runtime.effective(launch.Runtime)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrStartDenied, err)
	}
	if !engine.limiter.acquire(launch.Profile) {
		return "", ErrProcessLimitReached
	}
	releaseLimit := true
	defer func() {
		if releaseLimit {
			engine.limiter.release(launch.Profile)
		}
	}()

	process := &Process{
		state:        NewState(),
		output:       newOutputBuffer(),
		releaseLimit: func() { engine.limiter.release(launch.Profile) },
	}
	handle, err := engine.registry.Register(request.Principal, process)
	if err != nil {
		return "", err
	}

	started, err := engine.start(cloneLaunch(launch), process.output, process.output)
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
	process.setStarted(started)
	if err := process.state.Transition(StatusRunning); err != nil {
		return handle, fmt.Errorf("record running process: %w", err)
	}

	releaseLimit = false
	go reapProcess(process, started)
	go enforceRuntime(process, started, runtimeLimit)
	return handle, nil
}

// Stop terminates a principal-owned process by opaque handle. Already-terminal
// processes are returned unchanged, making repeated requests safe. Unknown and
// wrong-principal handles are indistinguishable. A successful kill attempts to
// record stopped; a terminal outcome concurrently recorded by the reaper wins.
func (engine *Engine) Stop(ctx context.Context, principal PrincipalID, handle Handle) (StopResult, error) {
	if ctx == nil || principal == "" || handle == "" {
		return StopResult{}, ErrInvalidStopRequest
	}
	if err := ctx.Err(); err != nil {
		return StopResult{}, fmt.Errorf("%w: %v", ErrInvalidStopRequest, err)
	}
	process, ok := engine.registry.Get(principal, handle)
	if !ok {
		return StopResult{}, ErrProcessUnavailable
	}

	process.control.Lock()
	defer process.control.Unlock()
	if status := process.Status(); status.Terminal() {
		return StopResult{Status: status, PID: process.PID()}, nil
	}
	started := process.startedProcess()
	if started == nil {
		return StopResult{}, ErrStopFailed
	}
	if err := started.Kill(); err != nil {
		if status := process.Status(); status.Terminal() {
			return StopResult{Status: status, PID: process.PID()}, nil
		}
		return StopResult{}, fmt.Errorf("%w: %v", ErrStopFailed, err)
	}
	if err := process.state.Transition(StatusStopped); err != nil {
		if status := process.Status(); status.Terminal() {
			return StopResult{Status: status, PID: process.PID()}, nil
		}
		return StopResult{}, fmt.Errorf("%w: %v", ErrStopFailed, err)
	}
	process.releaseBudget()
	return StopResult{Status: StatusStopped, PID: process.PID()}, nil
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

func startOSProcess(launch Launch, stdout, stderr io.Writer) (startedProcess, error) {
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Dir = launch.WorkingDirectory
	command.Env = append([]string(nil), launch.Environment...)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return osProcess{command}, nil
}

type osProcess struct{ command *exec.Cmd }

func (process osProcess) PID() int    { return process.command.Process.Pid }
func (process osProcess) Wait() error { return process.command.Wait() }
func (process osProcess) Kill() error { return process.command.Process.Kill() }

func reapProcess(process *Process, started startedProcess) {
	next := StatusExited
	if err := started.Wait(); err != nil {
		next = StatusFailed
	}
	process.releaseBudget()
	// A future stop/timeout path may win. State preserves the first terminal
	// outcome, so reaping cannot overwrite it.
	_ = process.state.Transition(next)
}

func enforceRuntime(process *Process, started startedProcess, limit time.Duration) {
	timer := time.NewTimer(limit)
	defer timer.Stop()

	select {
	case <-process.state.doneSignal():
		return
	case <-timer.C:
	}

	process.control.Lock()
	defer process.control.Unlock()
	if process.Status().Terminal() {
		return
	}
	if err := started.Kill(); err != nil {
		// A concurrently completed process is already represented by its reaper.
		// Otherwise leave the process nonterminal: a failed termination attempt
		// must never be reported as successful timeout enforcement.
		return
	}
	if err := process.state.Transition(StatusTimedOut); err == nil {
		process.releaseBudget()
	}
}
