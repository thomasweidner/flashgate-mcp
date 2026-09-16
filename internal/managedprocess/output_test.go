package managedprocess

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestEngineReadsOutputStreamsIndependently(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 101, wait: make(chan struct{})}
	engine.start = func(_ Launch, stdout, stderr io.Writer) (startedProcess, error) {
		_, _ = stdout.Write([]byte("alpha"))
		_, _ = stderr.Write([]byte("beta"))
		return started, nil
	}
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	first, err := engine.ReadOutput("owner", handle, OutputStdout, 0, 4)
	if err != nil || string(first.Data) != "alph" || first.Next != 4 || first.EOF || first.Truncated {
		t.Fatalf("stdout ReadOutput() = (%#v, %v)", first, err)
	}
	stderr, err := engine.ReadOutput("owner", handle, OutputStderr, 0, maxOutputRead)
	if err != nil || string(stderr.Data) != "beta" || stderr.Next != 4 || stderr.EOF || stderr.Truncated {
		t.Fatalf("stderr ReadOutput() = (%#v, %v)", stderr, err)
	}
	first.Data[0] = 'X'
	again, err := engine.ReadOutput("owner", handle, OutputStdout, 0, 4)
	if err != nil || string(again.Data) != "alph" {
		t.Fatalf("read result aliased capture: (%#v, %v)", again, err)
	}

	close(started.wait)
	waitForStatus(t, mustProcess(t, engine, "owner", handle), StatusExited)
	final, err := engine.ReadOutput("owner", handle, OutputStdout, first.Next, 1)
	if err != nil || string(final.Data) != "a" || final.Next != 5 || !final.EOF {
		t.Fatalf("final ReadOutput() = (%#v, %v)", final, err)
	}
}

func TestOutputRingEvictsOldestBytesAndMarksGap(t *testing.T) {
	buffer := newOutputBuffer(5)
	_, _ = buffer.Write([]byte("abc"))
	_, _ = buffer.Write([]byte("defg"))

	result, err := buffer.read(1, maxOutputRead, true)
	if err != nil || string(result.Data) != "cdefg" || result.Next != 7 || !result.EOF || !result.Truncated || result.DroppedBytes != 1 {
		t.Fatalf("ring read = (%#v, %v)", result, err)
	}
	continued, err := buffer.read(result.Next, 1, true)
	if err != nil || len(continued.Data) != 0 || continued.Truncated || !continued.EOF {
		t.Fatalf("continued read = (%#v, %v)", continued, err)
	}
}

func TestOutputRingHandlesWriteLargerThanLimit(t *testing.T) {
	buffer := newOutputBuffer(4)
	if written, err := buffer.Write([]byte("012345")); err != nil || written != 6 {
		t.Fatalf("Write() = (%d, %v)", written, err)
	}
	result, err := buffer.read(0, maxOutputRead, false)
	if err != nil || string(result.Data) != "2345" || result.Next != 6 || !result.Truncated || result.DroppedBytes != 2 {
		t.Fatalf("large-write read = (%#v, %v)", result, err)
	}
}

func TestEngineUsesIndependentConfiguredLimits(t *testing.T) {
	engine, err := NewEngineWithOutputLimits(policyFunc(allowTestLaunch), OutputLimits{Stdout: 3, Stderr: 5})
	if err != nil {
		t.Fatalf("NewEngineWithOutputLimits() error = %v", err)
	}
	started := &fakeStartedProcess{pid: 102, wait: make(chan struct{})}
	engine.start = func(_ Launch, stdout, stderr io.Writer) (startedProcess, error) {
		_, _ = stdout.Write([]byte("stdout"))
		_, _ = stderr.Write([]byte("stderr"))
		return started, nil
	}
	handle, startErr := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if startErr != nil {
		t.Fatalf("Start() error = %v", startErr)
	}
	defer close(started.wait)

	stdout, _ := engine.ReadOutput("owner", handle, OutputStdout, 0, maxOutputRead)
	stderr, _ := engine.ReadOutput("owner", handle, OutputStderr, 0, maxOutputRead)
	if string(stdout.Data) != "out" || stdout.DroppedBytes != 3 || string(stderr.Data) != "tderr" || stderr.DroppedBytes != 1 {
		t.Fatalf("configured rings: stdout=%#v stderr=%#v", stdout, stderr)
	}
}

func TestEngineReleaseOutputClearsBothStreamsAndKeepsDraining(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 103, wait: make(chan struct{})}
	var stdout, stderr io.Writer
	engine.start = func(_ Launch, out, errOut io.Writer) (startedProcess, error) {
		stdout, stderr = out, errOut
		_, _ = stdout.Write([]byte("secret-out"))
		_, _ = stderr.Write([]byte("secret-err"))
		return started, nil
	}
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer close(started.wait)
	if err := engine.ReleaseOutput("other", handle); !errors.Is(err, ErrProcessUnavailable) {
		t.Fatalf("wrong-owner ReleaseOutput() error = %v", err)
	}
	if err := engine.ReleaseOutput("owner", handle); err != nil {
		t.Fatalf("ReleaseOutput() error = %v", err)
	}
	if _, err := engine.ReadOutput("owner", handle, OutputStdout, 0, 1); !errors.Is(err, ErrOutputReleased) {
		t.Fatalf("released stdout error = %v", err)
	}
	if _, err := engine.ReadOutput("owner", handle, OutputStderr, 0, 1); !errors.Is(err, ErrOutputReleased) {
		t.Fatalf("released stderr error = %v", err)
	}
	if written, err := stdout.Write([]byte("later")); err != nil || written != 5 {
		t.Fatalf("released writer = (%d, %v)", written, err)
	}
}

func TestEngineReadOutputValidatesIdentityStreamBoundsAndCursor(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 104, wait: make(chan struct{})}
	engine.start = func(_ Launch, stdout, _ io.Writer) (startedProcess, error) {
		_, _ = stdout.Write([]byte("data"))
		return started, nil
	}
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer close(started.wait)

	tests := []struct {
		name      string
		principal PrincipalID
		handle    Handle
		stream    OutputStream
		maximum   int
		want      error
	}{
		{"missing principal", "", handle, OutputStdout, 1, ErrInvalidReadRequest},
		{"missing handle", "owner", "", OutputStdout, 1, ErrInvalidReadRequest},
		{"invalid stream", "owner", handle, "combined", 1, ErrInvalidReadRequest},
		{"zero maximum", "owner", handle, OutputStdout, 0, ErrInvalidReadRequest},
		{"oversize maximum", "owner", handle, OutputStdout, maxOutputRead + 1, ErrInvalidReadRequest},
		{"wrong owner", "other", handle, OutputStdout, 1, ErrProcessUnavailable},
		{"unknown handle", "owner", "unknown", OutputStdout, 1, ErrProcessUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, readErr := engine.ReadOutput(test.principal, test.handle, test.stream, 0, test.maximum); !errors.Is(readErr, test.want) {
				t.Fatalf("ReadOutput() error = %v, want %v", readErr, test.want)
			}
		})
	}
	if _, readErr := engine.ReadOutput("owner", handle, OutputStdout, 5, 1); !errors.Is(readErr, ErrOutputCursorUnavailable) {
		t.Fatalf("future cursor error = %v", readErr)
	}
}

func TestEngineRejectsInvalidOutputLimits(t *testing.T) {
	for _, limits := range []OutputLimits{{}, {Stdout: 1}, {Stderr: 1}, {Stdout: -1, Stderr: 1}} {
		if engine, err := NewEngineWithOutputLimits(policyFunc(allowTestLaunch), limits); engine != nil || !errors.Is(err, ErrInvalidOutputLimits) {
			t.Fatalf("NewEngineWithOutputLimits(%#v) = (%v, %v)", limits, engine, err)
		}
	}
}

func mustProcess(t *testing.T, engine *Engine, principal PrincipalID, handle Handle) *Process {
	t.Helper()
	process, ok := engine.Get(principal, handle)
	if !ok {
		t.Fatalf("process %q unavailable", handle)
	}
	return process
}
