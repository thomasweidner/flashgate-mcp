// Package operation provides transport-neutral lifecycle primitives for
// long-running, domain-owned work.
package operation

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const (
	handlePrefix       = "op_"
	handleEntropyBytes = 24
)

var (
	// ErrInvalidHandleBinding indicates that a required ownership field was
	// empty or was not in canonical form.
	ErrInvalidHandleBinding = errors.New("invalid operation handle binding")
	// ErrHandleGeneration indicates that secure handle entropy was unavailable.
	ErrHandleGeneration = errors.New("operation handle generation failed")
)

// HandleBinding is the authorization context captured when an operation
// handle is created. It is internal state and must not be encoded into or
// inferred from the public handle ID.
type HandleBinding struct {
	PrincipalID       string
	RootID            string
	ProfileID         string
	ExecutionBackend  string
	ServiceGeneration string
}

// Valid reports whether every ownership component is present and canonical.
func (b HandleBinding) Valid() bool {
	return canonicalIdentifier(b.PrincipalID) &&
		canonicalIdentifier(b.RootID) &&
		canonicalIdentifier(b.ProfileID) &&
		canonicalIdentifier(b.ExecutionBackend) &&
		canonicalIdentifier(b.ServiceGeneration)
}

func canonicalIdentifier(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}

// Handle is an opaque operation identifier and its server-side ownership
// binding. Callers may expose ID, but must keep Binding inside the trusted
// server boundary and repeat authorization checks before acting on the handle.
type Handle struct {
	id      string
	binding HandleBinding
}

// ID returns the opaque public operation identifier.
func (h Handle) ID() string {
	return h.id
}

// Binding returns the server-side ownership context associated with the
// handle. Returning a value prevents callers from mutating the stored binding.
func (h Handle) Binding() HandleBinding {
	return h.binding
}

// GenerateHandle creates an opaque, non-guessable operation handle. The ID
// contains only random data; ownership context remains server-side.
func GenerateHandle(binding HandleBinding) (Handle, error) {
	return generateHandle(binding, rand.Reader)
}

func generateHandle(binding HandleBinding, random io.Reader) (Handle, error) {
	if !binding.Valid() {
		return Handle{}, ErrInvalidHandleBinding
	}

	entropy := make([]byte, handleEntropyBytes)
	if _, err := io.ReadFull(random, entropy); err != nil {
		return Handle{}, ErrHandleGeneration
	}

	return Handle{
		id:      handlePrefix + base64.RawURLEncoding.EncodeToString(entropy),
		binding: binding,
	}, nil
}
