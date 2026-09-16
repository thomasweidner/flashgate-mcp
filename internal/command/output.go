package command

import (
	"errors"
	"sync"
)

// ErrInvalidOutputLimit indicates that a bounded output stream cannot be
// constructed from the configured limit.
var ErrInvalidOutputLimit = errors.New("output limit must be a positive supported byte count")

// OutputSnapshot is a point-in-time copy of one bounded process-output stream.
// TotalBytes counts all bytes presented to the stream, including bytes that
// could not be retained after the limit was reached.
type OutputSnapshot struct {
	Bytes      []byte
	TotalBytes int64
	Truncated  bool
}

// BoundedOutput keeps stdout and stderr in independent bounded buffers. It is
// safe for the two process-pipe drainers to write concurrently.
type BoundedOutput struct {
	Stdout *BoundedStream
	Stderr *BoundedStream
}

// NewBoundedOutput constructs separate stream buffers from validated command
// limits. Limits must be positive and fit in the platform's address space.
func NewBoundedOutput(stdoutLimit int64, stderrLimit int64) (*BoundedOutput, error) {
	stdout, err := NewBoundedStream(stdoutLimit)
	if err != nil {
		return nil, err
	}
	stderr, err := NewBoundedStream(stderrLimit)
	if err != nil {
		return nil, err
	}
	return &BoundedOutput{Stdout: stdout, Stderr: stderr}, nil
}

// BoundedStream implements io.Writer while retaining at most limit bytes. A
// write reports full consumption even when its tail is discarded so a process
// pipe can continue to be drained without output backpressure after truncation.
type BoundedStream struct {
	mu        sync.Mutex
	limit     int
	bytes     []byte
	total     int64
	truncated bool
}

// NewBoundedStream constructs a stream with a positive byte limit.
func NewBoundedStream(limit int64) (*BoundedStream, error) {
	if limit <= 0 || int64(int(limit)) != limit {
		return nil, ErrInvalidOutputLimit
	}
	return &BoundedStream{limit: int(limit), bytes: make([]byte, 0, min(int(limit), 4096))}, nil
}

// Write retains the earliest bytes up to the configured limit and accounts
// for all bytes received.
func (s *BoundedStream) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.total = saturatingAdd(s.total, len(p))
	remaining := s.limit - len(s.bytes)
	if remaining > 0 {
		retained := min(remaining, len(p))
		s.bytes = append(s.bytes, p[:retained]...)
	}
	if len(p) > remaining {
		s.truncated = true
	}
	return len(p), nil
}

// Snapshot returns an immutable copy suitable for later result construction.
func (s *BoundedStream) Snapshot() OutputSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return OutputSnapshot{
		Bytes:      append([]byte(nil), s.bytes...),
		TotalBytes: s.total,
		Truncated:  s.truncated,
	}
}

func saturatingAdd(total int64, added int) int64 {
	const maxInt64 = int64(^uint64(0) >> 1)
	if int64(added) > maxInt64-total {
		return maxInt64
	}
	return total + int64(added)
}
