// Package testconfig creates configuration sources for product tests.
package testconfig

import (
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/configfixture"
)

// Write creates a new fixture in a test-owned directory.
func Write(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := configfixture.Write(path, data); err != nil {
		t.Fatal(err)
	}
}
