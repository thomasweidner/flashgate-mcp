package ipc

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var wire bytes.Buffer
	want := Frame{Kind: KindRequest, Flags: 3, Payload: []byte(`{"id":1}`)}
	if err := WriteFrame(&wire, want, 128); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&wire, 128)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != want.Kind || got.Flags != want.Flags || !bytes.Equal(got.Payload, want.Payload) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestReadFrameRejectsLengthBeforeAllocation(t *testing.T) {
	var wire bytes.Buffer
	if err := WriteFrame(&wire, Frame{Kind: KindRequest, Payload: make([]byte, 9)}, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(&wire, 8); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("error = %v, want ErrFrameTooLarge", err)
	}
	if wire.Len() != 9 {
		t.Fatalf("payload was consumed after rejected length: %d bytes remain", wire.Len())
	}
}

func TestReadFrameRejectsInvalidHeader(t *testing.T) {
	for name, mutate := range map[string]func([]byte){
		"magic":   func(p []byte) { p[0] = 0 },
		"version": func(p []byte) { p[4] = 2 },
		"kind":    func(p []byte) { p[5] = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			wire := make([]byte, headerSize)
			copy(wire[:4], magic[:])
			wire[4], wire[5] = framingVersion, byte(KindRequest)
			mutate(wire)
			if _, err := ReadFrame(bytes.NewReader(wire), 1); !errors.Is(err, ErrInvalidFrame) {
				t.Fatalf("error = %v, want ErrInvalidFrame", err)
			}
		})
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > 2 {
		p = p[:2]
	}
	return w.Buffer.Write(p)
}

func TestWriteFrameHandlesShortWrites(t *testing.T) {
	var wire shortWriter
	if err := WriteFrame(&wire, Frame{Kind: KindClose, Payload: []byte("bye")}, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(bytes.NewReader(wire.Bytes()), 3); err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
