//go:build linux

// Package unixsocket provides the Linux local IPC transport boundary.
package unixsocket

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

var (
	// ErrEndpointInUse means a live listener already owns the configured path.
	ErrEndpointInUse = errors.New("unix socket endpoint is already in use")
	// ErrUnsafeEndpoint means the path or requested permissions are unsafe.
	ErrUnsafeEndpoint = errors.New("unsafe unix socket endpoint")
	// ErrPeerCredentials means authoritative kernel peer credentials were unavailable.
	ErrPeerCredentials = errors.New("unix socket peer credentials unavailable")
)

// Options controls creation of a filesystem-backed Unix Domain Socket.
type Options struct {
	Path string
	Mode fs.FileMode
}

// PeerCredentials are the authoritative credentials returned by SO_PEERCRED.
type PeerCredentials struct {
	PID int32
	UID uint32
	GID uint32
}

// Conn is an accepted local connection with immutable kernel credentials.
type Conn struct {
	*net.UnixConn
	peer PeerCredentials
}

// PeerCredentials returns the credentials captured before the connection was exposed.
func (c *Conn) PeerCredentials() PeerCredentials { return c.peer }

// Listener owns one filesystem-backed local endpoint.
type Listener struct {
	listener *net.UnixListener
	path     string
	endpoint fs.FileInfo
	close    sync.Once
	closeErr error
}

// Listen creates a local-only Unix Domain Socket. A stale socket is removed only
// after a connection probe proves that no listener answers at the same path.
func Listen(options Options) (*Listener, error) {
	path, mode, err := validateOptions(options)
	if err != nil {
		return nil, err
	}

	if err := prepareEndpoint(path); err != nil {
		return nil, err
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("create unix socket: %w", err)
	}
	listener.SetUnlinkOnClose(true)

	if err := os.Chmod(path, mode); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("set unix socket permissions: %w", err)
	}
	endpoint, err := os.Lstat(path)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("verify unix socket endpoint: %w", err)
	}
	endpointStat, ok := endpoint.Sys().(*syscall.Stat_t)
	if !ok || endpointStat.Uid != uint32(os.Geteuid()) || endpointStat.Gid != uint32(os.Getegid()) {
		_ = listener.Close()
		return nil, fmt.Errorf("%w: socket ownership differs from the service identity", ErrUnsafeEndpoint)
	}
	listener.SetUnlinkOnClose(false)

	return &Listener{listener: listener, path: path, endpoint: endpoint}, nil
}

func validateOptions(options Options) (string, fs.FileMode, error) {
	if options.Path == "" || !filepath.IsAbs(options.Path) || filepath.Clean(options.Path) != options.Path {
		return "", 0, fmt.Errorf("%w: path must be a clean absolute filesystem path", ErrUnsafeEndpoint)
	}
	if len(options.Path) >= len(syscall.RawSockaddrUnix{}.Path) {
		return "", 0, fmt.Errorf("%w: path exceeds the platform limit", ErrUnsafeEndpoint)
	}

	mode := options.Mode
	if mode == 0 {
		mode = 0o600
	}
	if mode&^fs.FileMode(0o660) != 0 || mode&0o600 != 0o600 || mode&0o007 != 0 {
		return "", 0, fmt.Errorf("%w: mode must grant owner read/write, no access to others, and at most group read/write", ErrUnsafeEndpoint)
	}

	parent, err := os.Lstat(filepath.Dir(options.Path))
	if err != nil {
		return "", 0, fmt.Errorf("inspect unix socket directory: %w", err)
	}
	if !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
		return "", 0, fmt.Errorf("%w: parent must be a real directory", ErrUnsafeEndpoint)
	}
	parentStat, ok := parent.Sys().(*syscall.Stat_t)
	if !ok || parentStat.Uid != uint32(os.Geteuid()) || parent.Mode().Perm()&0o022 != 0 {
		return "", 0, fmt.Errorf("%w: parent must be owned by the effective user and not group/other writable", ErrUnsafeEndpoint)
	}

	return options.Path, mode, nil
}

func prepareEndpoint(path string) error {
	first, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect unix socket endpoint: %w", err)
	}
	if first.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%w: existing endpoint is not a socket", ErrUnsafeEndpoint)
	}

	probe, dialErr := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if dialErr == nil {
		_ = probe.Close()
		return ErrEndpointInUse
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("probe existing unix socket endpoint: %w", dialErr)
	}

	second, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("reinspect stale unix socket endpoint: %w", err)
	}
	if second.Mode()&os.ModeSocket == 0 || !os.SameFile(first, second) {
		return fmt.Errorf("%w: endpoint changed during stale-socket check", ErrUnsafeEndpoint)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale unix socket endpoint: %w", err)
	}
	return nil
}

// Accept authenticates the next connection through Linux SO_PEERCRED before
// returning it to application code.
func (l *Listener) Accept() (*Conn, error) {
	conn, err := l.listener.AcceptUnix()
	if err != nil {
		return nil, err
	}

	peer, err := peerCredentials(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Conn{UnixConn: conn, peer: peer}, nil
}

// Addr returns the local endpoint address.
func (l *Listener) Addr() net.Addr { return l.listener.Addr() }

// Close closes the listener and removes the endpoint only when it is still the
// same filesystem object created by Listen.
func (l *Listener) Close() error {
	l.close.Do(func() {
		l.closeErr = l.listener.Close()
		current, err := os.Lstat(l.path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			l.closeErr = errors.Join(l.closeErr, fmt.Errorf("inspect unix socket during cleanup: %w", err))
		case !os.SameFile(l.endpoint, current):
			l.closeErr = errors.Join(l.closeErr, fmt.Errorf("%w: endpoint changed before cleanup", ErrUnsafeEndpoint))
		default:
			l.closeErr = errors.Join(l.closeErr, os.Remove(l.path))
		}
	})
	return l.closeErr
}

func peerCredentials(conn *net.UnixConn) (PeerCredentials, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return PeerCredentials{}, fmt.Errorf("%w: %v", ErrPeerCredentials, err)
	}

	var credentials *syscall.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		credentials, socketErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return PeerCredentials{}, fmt.Errorf("%w: %v", ErrPeerCredentials, err)
	}
	if socketErr != nil || credentials == nil || credentials.Pid <= 0 {
		return PeerCredentials{}, fmt.Errorf("%w: kernel query failed", ErrPeerCredentials)
	}

	return PeerCredentials{PID: credentials.Pid, UID: credentials.Uid, GID: credentials.Gid}, nil
}
