//go:build windows

// Package namedpipe provides the local-only Windows transport for FlashGate.
package namedpipe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/thomasweidner/flashgate-mcp/internal/ipc"
)

const (
	pipeAccessDuplex        = 0x00000003
	pipeTypeByte            = 0x00000000
	pipeReadmodeByte        = 0x00000000
	pipeWait                = 0x00000000
	pipeRejectRemoteClients = 0x00000008
	sddlRevision            = 1
	errorPipeConnected      = syscall.Errno(535)
)

// The DACL is protected from inheritance. SYSTEM and Administrators retain
// full control; authenticated local users may connect, with authorization
// decided later from the OS-derived SID.
const endpointSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)"

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	advapi32                   = syscall.NewLazyDLL("advapi32.dll")
	createNamedPipe            = kernel32.NewProc("CreateNamedPipeW")
	connectNamedPipe           = kernel32.NewProc("ConnectNamedPipe")
	disconnectNamedPipe        = kernel32.NewProc("DisconnectNamedPipe")
	closeHandle                = kernel32.NewProc("CloseHandle")
	localFree                  = kernel32.NewProc("LocalFree")
	impersonateNamedPipeClient = advapi32.NewProc("ImpersonateNamedPipeClient")
	revertToSelf               = advapi32.NewProc("RevertToSelf")
	openThreadToken            = advapi32.NewProc("OpenThreadToken")
	convertSDDL                = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
)

// Config contains bounded transport settings.
type Config struct {
	Name              string
	MaxPayloadBytes   uint64
	InputBufferBytes  uint32
	OutputBufferBytes uint32
}

// PeerIdentity is obtained from the connected client's OS token. It contains
// no identity asserted by protocol payloads.
type PeerIdentity struct{ SID string }

// Listener accepts one local Named Pipe connection per pipe instance.
type Listener struct {
	config   Config
	acceptMu sync.Mutex
	mu       sync.Mutex
	closed   bool
	pending  syscall.Handle
}

// Listen validates config. Pipe instances are created lazily by Accept.
func Listen(config Config) (*Listener, error) {
	if !validName(config.Name) {
		return nil, errors.New("invalid local pipe name")
	}
	if config.MaxPayloadBytes == 0 || config.InputBufferBytes == 0 || config.OutputBufferBytes == 0 {
		return nil, errors.New("named pipe limits must be positive")
	}
	return &Listener{config: config}, nil
}

func validName(name string) bool {
	const prefix = `\\.\pipe\`
	if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
		return false
	}
	tail := name[len(prefix):]
	return tail != "" && !strings.ContainsAny(tail, "/\\\x00")
}

// Accept waits for one client, authenticates it from its impersonation token,
// and returns a framed connection. Cancelling ctx closes only this pending pipe
// instance and leaves the listener usable.
func (l *Listener) Accept(ctx context.Context) (*Conn, error) {
	l.acceptMu.Lock()
	defer l.acceptMu.Unlock()
	h, err := l.newInstance()
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		closeHandle.Call(uintptr(h))
		return nil, os.ErrClosed
	}
	l.pending = h
	l.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		r, _, callErr := connectNamedPipe.Call(uintptr(h), 0)
		if r == 0 && !errors.Is(callErr, errorPipeConnected) {
			done <- callErr
			return
		}
		done <- nil
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		if l.clearPending(h) {
			closeHandle.Call(uintptr(h))
		}
		<-done
		err = ctx.Err()
	}
	ownsHandle := l.clearPending(h)
	if err != nil {
		if ownsHandle {
			closeHandle.Call(uintptr(h))
		}
		return nil, fmt.Errorf("accept named pipe: %w", err)
	}
	if !ownsHandle {
		return nil, os.ErrClosed
	}

	identity, err := peerIdentity(h)
	if err != nil {
		disconnectNamedPipe.Call(uintptr(h))
		closeHandle.Call(uintptr(h))
		return nil, err
	}
	file := os.NewFile(uintptr(h), l.config.Name)
	if file == nil {
		closeHandle.Call(uintptr(h))
		return nil, errors.New("create named pipe file")
	}
	return &Conn{file: file, identity: identity, maxPayload: l.config.MaxPayloadBytes}, nil
}

func (l *Listener) newInstance() (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(l.config.Name)
	if err != nil {
		return 0, errors.New("invalid local pipe name")
	}
	sa, free, err := securityAttributes()
	if err != nil {
		return 0, err
	}
	defer free()
	r, _, callErr := createNamedPipe.Call(uintptr(unsafe.Pointer(name)), pipeAccessDuplex,
		pipeTypeByte|pipeReadmodeByte|pipeWait|pipeRejectRemoteClients, 255,
		uintptr(l.config.OutputBufferBytes), uintptr(l.config.InputBufferBytes), 0, uintptr(unsafe.Pointer(sa)))
	if r == uintptr(syscall.InvalidHandle) {
		return 0, fmt.Errorf("create named pipe: %w", callErr)
	}
	return syscall.Handle(r), nil
}

func (l *Listener) clearPending(h syscall.Handle) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pending == h {
		l.pending = 0
		return true
	}
	return false
}

// Close cancels a pending Accept.
func (l *Listener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	pending := l.pending
	l.pending = 0
	l.mu.Unlock()
	if pending != 0 {
		closeHandle.Call(uintptr(pending))
	}
	return nil
}

// Conn is an authenticated, bounded local IPC connection.
type Conn struct {
	file       *os.File
	identity   PeerIdentity
	maxPayload uint64
	readMu     sync.Mutex
	writeMu    sync.Mutex
}

func (c *Conn) PeerIdentity() PeerIdentity { return c.identity }
func (c *Conn) ReadFrame() (ipc.Frame, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	return ipc.ReadFrame(c.file, c.maxPayload)
}
func (c *Conn) WriteFrame(frame ipc.Frame) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return ipc.WriteFrame(c.file, frame, c.maxPayload)
}
func (c *Conn) Close() error { disconnectNamedPipe.Call(c.file.Fd()); return c.file.Close() }

func securityAttributes() (*syscall.SecurityAttributes, func(), error) {
	sddl, _ := syscall.UTF16PtrFromString(endpointSDDL)
	var descriptor uintptr
	r, _, err := convertSDDL.Call(uintptr(unsafe.Pointer(sddl)), sddlRevision, uintptr(unsafe.Pointer(&descriptor)), 0)
	if r == 0 {
		return nil, func() {}, fmt.Errorf("create named pipe security descriptor: %w", err)
	}
	sa := &syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), SecurityDescriptor: descriptor, InheritHandle: 0}
	return sa, func() { localFree.Call(descriptor) }, nil
}

func peerIdentity(pipe syscall.Handle) (_ PeerIdentity, err error) {
	r, _, callErr := impersonateNamedPipeClient.Call(uintptr(pipe))
	if r == 0 {
		return PeerIdentity{}, fmt.Errorf("authenticate named pipe peer: %w", callErr)
	}
	defer func() {
		if reverted, _, revertErr := revertToSelf.Call(); reverted == 0 && err == nil {
			err = fmt.Errorf("restore service identity: %w", revertErr)
		}
	}()
	var token syscall.Token
	r, _, callErr = openThreadToken.Call(^uintptr(1), syscall.TOKEN_QUERY, 1, uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		return PeerIdentity{}, fmt.Errorf("read named pipe peer token: %w", callErr)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return PeerIdentity{}, fmt.Errorf("read named pipe peer SID: %w", err)
	}
	sid, err := user.User.Sid.String()
	if err != nil {
		return PeerIdentity{}, fmt.Errorf("format named pipe peer SID: %w", err)
	}
	return PeerIdentity{SID: sid}, nil
}

var _ io.Closer = (*Conn)(nil)
