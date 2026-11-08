package transport

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/thomasweidner/flashgate-mcp/internal/wiretrace"
)

var (
	// ErrMessageTooLarge is returned when a JSON-RPC message exceeds the configured read limit.
	ErrMessageTooLarge = errors.New("message exceeds maximum allowed size")

	// ErrResponseTooLarge is returned when a JSON-RPC response exceeds the configured write limit.
	ErrResponseTooLarge = errors.New("response exceeds maximum allowed size")
)

// Transport handles line-based JSON communication over readers and writers.
type Transport struct {
	in            *bufio.Reader
	out           *bufio.Writer
	maxReadBytes  int64
	maxWriteBytes int64
}

// New creates a new transport.
func New(in io.Reader, out io.Writer) *Transport {
	return NewWithLimits(in, out, 0, 0)
}

// NewWithLimits creates a new transport with optional byte limits.
func NewWithLimits(in io.Reader, out io.Writer, maxReadBytes int64, maxWriteBytes int64) *Transport {
	writer := bufio.NewWriter(out)

	return &Transport{
		in:            bufio.NewReader(in),
		out:           writer,
		maxReadBytes:  maxReadBytes,
		maxWriteBytes: maxWriteBytes,
	}
}

// ReadMessage reads a single JSON message.
func (t *Transport) ReadMessage() ([]byte, error) {
	line := make([]byte, 0, 1024)

	for {
		next, err := t.in.ReadByte()
		if err != nil {
			return nil, err
		}

		line = append(line, next)
		if t.maxReadBytes > 0 && int64(len(line)) > t.maxReadBytes {
			t.discardLine()
			return nil, ErrMessageTooLarge
		}

		if next == '\n' {
			return line, nil
		}
	}
}

// WriteMessage writes a single JSON message.
func (t *Transport) WriteMessage(value any) error {
	return t.WriteMessageContext(context.Background(), value)
}

// WriteMessageContext passes a request-scoped serialization observer, if any.
func (t *Transport) WriteMessageContext(ctx context.Context, value any) error {
	return t.writeMessage(ctx, value, true)
}

// WriteMessageUnbounded writes a JSON message without applying the response size limit.
func (t *Transport) WriteMessageUnbounded(value any) error {
	return t.writeMessage(context.Background(), value, false)
}

func (t *Transport) writeMessage(ctx context.Context, value any, enforceLimit bool) error {
	payload, err := wiretrace.Marshal(ctx, value)
	if err != nil {
		return err
	}

	if enforceLimit && t.maxWriteBytes > 0 && int64(len(payload)+1) > t.maxWriteBytes {
		return ErrResponseTooLarge
	}

	if _, err := t.out.Write(payload); err != nil {
		return err
	}

	if err := t.out.WriteByte('\n'); err != nil {
		return err
	}

	return t.out.Flush()
}

func (t *Transport) discardLine() {
	for {
		next, err := t.in.ReadByte()
		if err != nil || next == '\n' {
			return
		}
	}
}
