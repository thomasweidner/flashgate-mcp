// Package ipc implements the bounded framing shared by local IPC transports.
package ipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	headerSize     = 16
	framingVersion = 1
)

var magic = [4]byte{'F', 'G', 'I', 'P'}

// Kind identifies a Version 1 local IPC frame.
type Kind uint8

const (
	KindClientHello Kind = 1 + iota
	KindServerHello
	KindRequest
	KindResponse
	KindCancel
	KindEvent
	KindClose
	KindHeartbeat
)

var (
	ErrInvalidFrame  = errors.New("invalid local IPC frame")
	ErrFrameTooLarge = errors.New("local IPC frame exceeds configured limit")
)

// Frame is one complete local IPC message. Payload is never fragmented by the
// framing layer.
type Frame struct {
	Kind    Kind
	Flags   uint16
	Payload []byte
}

// ReadFrame reads one frame while enforcing maxPayload before allocation.
func ReadFrame(r io.Reader, maxPayload uint64) (Frame, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Frame{}, err
	}
	if [4]byte(header[:4]) != magic || header[4] != framingVersion {
		return Frame{}, ErrInvalidFrame
	}
	kind := Kind(header[5])
	if kind < KindClientHello || kind > KindHeartbeat {
		return Frame{}, ErrInvalidFrame
	}
	length := binary.BigEndian.Uint64(header[8:])
	if maxPayload == 0 || length > maxPayload || length > uint64(int(^uint(0)>>1)) {
		return Frame{}, ErrFrameTooLarge
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(r, payload); err != nil {
		return Frame{}, err
	}
	return Frame{Kind: kind, Flags: binary.BigEndian.Uint16(header[6:8]), Payload: payload}, nil
}

// WriteFrame writes one complete frame while enforcing maxPayload.
func WriteFrame(w io.Writer, frame Frame, maxPayload uint64) error {
	if frame.Kind < KindClientHello || frame.Kind > KindHeartbeat {
		return ErrInvalidFrame
	}
	if maxPayload == 0 || uint64(len(frame.Payload)) > maxPayload {
		return ErrFrameTooLarge
	}
	var header [headerSize]byte
	copy(header[:4], magic[:])
	header[4] = framingVersion
	header[5] = byte(frame.Kind)
	binary.BigEndian.PutUint16(header[6:8], frame.Flags)
	binary.BigEndian.PutUint64(header[8:], uint64(len(frame.Payload)))
	if err := writeAll(w, header[:]); err != nil {
		return fmt.Errorf("write local IPC header: %w", err)
	}
	if err := writeAll(w, frame.Payload); err != nil {
		return fmt.Errorf("write local IPC payload: %w", err)
	}
	return nil
}

func writeAll(w io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := w.Write(payload)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		payload = payload[n:]
	}
	return nil
}
