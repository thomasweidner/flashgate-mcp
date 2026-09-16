package managedprocess

import (
	"errors"
	"sync"
)

const (
	defaultOutputLimit = 1024 * 1024
	maxOutputRead      = 64 * 1024
)

var (
	ErrOutputCursorUnavailable = errors.New("managed process output cursor unavailable")
	ErrOutputReleased          = errors.New("managed process output released")
	ErrInvalidOutputLimits     = errors.New("invalid managed process output limits")
)

// OutputStream identifies one independently captured child-process stream.
type OutputStream string

const (
	OutputStdout OutputStream = "stdout"
	OutputStderr OutputStream = "stderr"
)

// OutputLimits bounds retained bytes independently for stdout and stderr.
type OutputLimits struct {
	Stdout int
	Stderr int
}

func (limits OutputLimits) valid() bool { return limits.Stdout > 0 && limits.Stderr > 0 }

// OutputCursor is an absolute byte position in one process stream. It has no
// authority by itself: every read is resolved through the principal-bound
// process handle and its selected stream.
type OutputCursor uint64

// OutputResult is one bounded, incremental output page. When a requested
// cursor has fallen behind the ring, Truncated is true and DroppedBytes says
// how many bytes were skipped before Data begins.
type OutputResult struct {
	Data         []byte
	Next         OutputCursor
	EOF          bool
	Truncated    bool
	DroppedBytes uint64
}

// outputBuffer retains the newest bounded suffix of one output stream.
type outputBuffer struct {
	mu       sync.RWMutex
	data     []byte
	start    OutputCursor
	end      OutputCursor
	limit    int
	released bool
}

func newOutputBuffer(limit int) *outputBuffer {
	capacity := min(limit, 4096)
	return &outputBuffer{data: make([]byte, 0, capacity), limit: limit}
}

func (buffer *outputBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	if buffer.released {
		return len(data), nil
	}
	buffer.end += OutputCursor(len(data))
	if len(data) >= buffer.limit {
		clear(buffer.data)
		buffer.data = append(buffer.data[:0], data[len(data)-buffer.limit:]...)
		buffer.start = buffer.end - OutputCursor(buffer.limit)
		return len(data), nil
	}

	overflow := len(buffer.data) + len(data) - buffer.limit
	if overflow > 0 {
		clear(buffer.data[:overflow])
		copy(buffer.data, buffer.data[overflow:])
		buffer.data = buffer.data[:len(buffer.data)-overflow]
		buffer.start += OutputCursor(overflow)
	}
	buffer.data = append(buffer.data, data...)
	return len(data), nil
}

func (buffer *outputBuffer) read(cursor OutputCursor, maximum int, terminal bool) (OutputResult, error) {
	if maximum <= 0 || maximum > maxOutputRead {
		return OutputResult{}, ErrInvalidReadRequest
	}

	buffer.mu.RLock()
	defer buffer.mu.RUnlock()
	if buffer.released {
		return OutputResult{}, ErrOutputReleased
	}
	if cursor > buffer.end {
		return OutputResult{}, ErrOutputCursorUnavailable
	}

	result := OutputResult{}
	if cursor < buffer.start {
		result.Truncated = true
		result.DroppedBytes = uint64(buffer.start - cursor)
		cursor = buffer.start
	}
	available := int(buffer.end - cursor)
	if available > maximum {
		available = maximum
	}
	offset := int(cursor - buffer.start)
	result.Data = append([]byte(nil), buffer.data[offset:offset+available]...)
	result.Next = cursor + OutputCursor(available)
	result.EOF = terminal && result.Next == buffer.end
	return result, nil
}

func (buffer *outputBuffer) release() {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	clear(buffer.data)
	buffer.data = nil
	buffer.released = true
}

// ReadOutput returns the next bounded page from one output stream. Unknown and
// wrong-principal handles are indistinguishable. A zero cursor starts at the
// oldest retained byte and reports any bytes already evicted from the ring.
func (engine *Engine) ReadOutput(principal PrincipalID, handle Handle, stream OutputStream, cursor OutputCursor, maximum int) (OutputResult, error) {
	if principal == "" || handle == "" {
		return OutputResult{}, ErrInvalidReadRequest
	}
	process, ok := engine.registry.Get(principal, handle)
	if !ok {
		return OutputResult{}, ErrProcessUnavailable
	}
	buffer, ok := process.output.stream(stream)
	if !ok {
		return OutputResult{}, ErrInvalidReadRequest
	}
	return buffer.read(cursor, maximum, process.Status().Terminal())
}

// ReleaseOutput securely discards both captured streams while leaving the
// process lifecycle record intact. Writers continue to drain successfully so
// releasing output cannot block or fail a running child.
func (engine *Engine) ReleaseOutput(principal PrincipalID, handle Handle) error {
	if principal == "" || handle == "" {
		return ErrInvalidReadRequest
	}
	process, ok := engine.registry.Get(principal, handle)
	if !ok {
		return ErrProcessUnavailable
	}
	process.output.release()
	return nil
}
