//go:build !windows

// Package testconfig creates configuration sources for product tests.
package testconfig

import (
	"os"
	"testing"
)

// Write creates a new configuration fixture with owner-only permissions.
func Write(t testing.TB, path string, data []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal("creating configuration fixture failed")
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("writing configuration fixture failed")
	}
}
