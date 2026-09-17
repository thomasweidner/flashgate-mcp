//go:build linux

package unixsocket

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestListenerAcceptsLocalPeerWithKernelCredentials(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "flashgate.sock")
	listener, err := Listen(Options{Path: path, Mode: 0o660})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode().Perm() != 0o660 || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket mode = %v, want socket 0660", info.Mode())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) {
		t.Fatalf("socket ownership = %#v, want uid=%d gid=%d", info.Sys(), os.Geteuid(), os.Getegid())
	}

	accepted := make(chan *Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()

	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer client.Close()

	select {
	case err := <-acceptErr:
		t.Fatalf("Accept() error = %v", err)
	case conn := <-accepted:
		defer conn.Close()
		peer := conn.PeerCredentials()
		if peer.PID != int32(os.Getpid()) || peer.UID != uint32(os.Getuid()) || peer.GID != uint32(os.Getgid()) {
			t.Fatalf("peer credentials = %+v, want pid=%d uid=%d gid=%d", peer, os.Getpid(), os.Getuid(), os.Getgid())
		}
	}
}

func TestListenerRejectsActiveEndpoint(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "flashgate.sock")
	first, err := Listen(Options{Path: path})
	if err != nil {
		t.Fatalf("first Listen() error = %v", err)
	}
	defer first.Close()

	_, err = Listen(Options{Path: path})
	if !errors.Is(err, ErrEndpointInUse) {
		t.Fatalf("second Listen() error = %v, want ErrEndpointInUse", err)
	}
}

func TestListenerReplacesStaleSocketAndCleansUp(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "flashgate.sock")
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	stale.SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatalf("close stale socket: %v", err)
	}

	listener, err := Listen(Options{Path: path})
	if err != nil {
		t.Fatalf("Listen() with stale endpoint error = %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("endpoint after Close() error = %v, want not exist", err)
	}
}

func TestListenerRejectsUnsafePathAndMode(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	regular := filepath.Join(directory, "regular")
	if err := os.WriteFile(regular, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []Options{
		{Path: "relative.sock"},
		{Path: filepath.Join(directory, "world.sock"), Mode: 0o666},
		{Path: regular},
	}
	for _, options := range tests {
		if _, err := Listen(options); !errors.Is(err, ErrUnsafeEndpoint) {
			t.Errorf("Listen(%+v) error = %v, want ErrUnsafeEndpoint", options, err)
		}
	}

	content, err := os.ReadFile(regular)
	if err != nil || string(content) != "preserve" {
		t.Fatalf("regular endpoint changed: content=%q error=%v", content, err)
	}
}

func TestListenerClosePreservesReplacementEndpoint(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "flashgate.sock")
	listener, err := Listen(Options{Path: path})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove owned endpoint: %v", err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
		t.Fatalf("create replacement: %v", err)
	}

	if err := listener.Close(); !errors.Is(err, ErrUnsafeEndpoint) {
		t.Fatalf("Close() error = %v, want ErrUnsafeEndpoint", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "replacement" {
		t.Fatalf("replacement changed: content=%q error=%v", content, err)
	}
}
