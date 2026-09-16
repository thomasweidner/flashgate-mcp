package managedprocess

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestEngineReadsOutputIncrementally(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 101, wait: make(chan struct{})}
	engine.start = func(_ Launch, stdout, stderr io.Writer) (startedProcess, error) {
		_, _ = stdout.Write([]byte("alpha"))
		_, _ = stderr.Write([]byte("-beta"))
		return started, nil
	}
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	first, err := engine.ReadOutput("owner", handle, 0, 4)
	if err != nil || string(first.Data) != "alph" || first.Next != 4 || first.EOF || first.Truncated {
		t.Fatalf("first ReadOutput() = (%#v, %v)", first, err)
	}
	first.Data[0] = 'X'
	again, err := engine.ReadOutput("owner", handle, 0, 4)
	if err != nil || string(again.Data) != "alph" {
		t.Fatalf("read result aliased capture: (%#v, %v)", again, err)
	}
	second, err := engine.ReadOutput("owner", handle, first.Next, maxOutputRead)
	if err != nil || string(second.Data) != "a-beta" || second.Next != 10 || second.EOF || second.Truncated {
		t.Fatalf("second ReadOutput() = (%#v, %v)", second, err)
	}

	close(started.wait)
	waitForStatus(t, mustProcess(t, engine, "owner", handle), StatusExited)
	final, err := engine.ReadOutput("owner", handle, second.Next, 1)
	if err != nil || len(final.Data) != 0 || final.Next != second.Next || !final.EOF {
		t.Fatalf("final ReadOutput() = (%#v, %v)", final, err)
	}
}

func TestEngineReadOutputValidatesIdentityBoundsAndCursor(t *testing.T) {
	engine := NewEngine(policyFunc(allowTestLaunch))
	started := &fakeStartedProcess{pid: 102, wait: make(chan struct{})}
	engine.start = func(_ Launch, stdout, _ io.Writer) (startedProcess, error) {
		_, _ = stdout.Write([]byte("data"))
		return started, nil
	}
	handle, err := engine.Start(context.Background(), StartRequest{Principal: "owner", Command: "approved"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer close(started.wait)

	for _, test := range []struct {
		name      string
		principal PrincipalID
		handle    Handle
		maximum   int
		want      error
	}{
		{"missing principal", "", handle, 1, ErrInvalidReadRequest},
		{"missing handle", "owner", "", 1, ErrInvalidReadRequest},
		{"zero maximum", "owner", handle, 0, ErrInvalidReadRequest},
		{"oversize maximum", "owner", handle, maxOutputRead + 1, ErrInvalidReadRequest},
		{"wrong owner", "other", handle, 1, ErrProcessUnavailable},
		{"unknown handle", "owner", "unknown", 1, ErrProcessUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, readErr := engine.ReadOutput(test.principal, test.handle, 0, test.maximum); !errors.Is(readErr, test.want) {
				t.Fatalf("ReadOutput() error = %v, want %v", readErr, test.want)
			}
		})
	}
	if _, readErr := engine.ReadOutput("owner", handle, 5, 1); !errors.Is(readErr, ErrOutputCursorUnavailable) {
		t.Fatalf("future cursor error = %v, want %v", readErr, ErrOutputCursorUnavailable)
	}
}

func TestOutputBufferCapsCaptureAndReportsTruncation(t *testing.T) {
	buffer := newOutputBuffer()
	payload := make([]byte, maxCapturedOutput+17)
	for index := range payload {
		payload[index] = 'x'
	}
	if written, err := buffer.Write(payload); err != nil || written != len(payload) {
		t.Fatalf("Write() = (%d, %v), want (%d, nil)", written, err, len(payload))
	}
	result, err := buffer.read(OutputCursor(maxCapturedOutput-2), maxOutputRead, true)
	if err != nil || string(result.Data) != "xx" || result.Next != maxCapturedOutput || !result.EOF || !result.Truncated {
		t.Fatalf("truncated read = (%#v, %v)", result, err)
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
