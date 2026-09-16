package managedprocess

import (
	"errors"
	"sync"
)

const (
	// The engine-level capture is deliberately finite even before the separate
	// stream/ring-buffer policy is added. BL-125 will replace this shared
	// capture with independently configured stdout and stderr rings.
	maxCapturedOutput = 1024 * 1024
	maxOutputRead     = 64 * 1024
)

var ErrOutputCursorUnavailable = errors.New("managed process output cursor unavailable")

// OutputCursor is a byte position in one process's combined output. It has no
// authority by itself: every read is resolved through the principal-bound
// process handle.
type OutputCursor uint64

// OutputResult is one bounded, incremental output page.
type OutputResult struct {
	Data      []byte
	Next      OutputCursor
	EOF       bool
	Truncated bool
}

// outputBuffer is the initial shared capture used by read_process_output.
// It retains the first bounded prefix. Separate per-stream ring semantics,
// configurable limits, and cleanup belong to BL-125.
type outputBuffer struct {
	mu        sync.RWMutex
	data      []byte
	truncated bool
}

func newOutputBuffer() *outputBuffer {
	return &outputBuffer{data: make([]byte, 0, 4096)}
}

func (buffer *outputBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	remaining := maxCapturedOutput - len(buffer.data)
	if remaining < len(data) {
		buffer.truncated = true
	}
	if remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		buffer.data = append(buffer.data, data[:remaining]...)
	}
	// Writers must observe successful consumption after the safe prefix fills;
	// otherwise an os/exec pipe can turn the output limit into process failure.
	return len(data), nil
}

func (buffer *outputBuffer) read(cursor OutputCursor, maximum int, terminal bool) (OutputResult, error) {
	if maximum <= 0 || maximum > maxOutputRead {
		return OutputResult{}, ErrInvalidReadRequest
	}

	buffer.mu.RLock()
	defer buffer.mu.RUnlock()

	if cursor > OutputCursor(len(buffer.data)) {
		return OutputResult{}, ErrOutputCursorUnavailable
	}
	end := int(cursor) + maximum
	if end > len(buffer.data) {
		end = len(buffer.data)
	}
	data := append([]byte(nil), buffer.data[int(cursor):end]...)
	next := OutputCursor(end)
	return OutputResult{
		Data:      data,
		Next:      next,
		EOF:       terminal && next == OutputCursor(len(buffer.data)),
		Truncated: buffer.truncated,
	}, nil
}

// ReadOutput returns the next bounded page of output for a principal-owned
// managed process. Unknown and wrong-principal handles are indistinguishable.
// A zero cursor starts at the beginning; the returned cursor continues the
// same process output without retransmitting prior bytes.
func (engine *Engine) ReadOutput(principal PrincipalID, handle Handle, cursor OutputCursor, maximum int) (OutputResult, error) {
	if principal == "" || handle == "" {
		return OutputResult{}, ErrInvalidReadRequest
	}
	process, ok := engine.registry.Get(principal, handle)
	if !ok {
		return OutputResult{}, ErrProcessUnavailable
	}
	return process.output.read(cursor, maximum, process.Status().Terminal())
}
