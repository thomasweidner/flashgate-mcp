// Package resultresource defines the transport-neutral identity and handoff
// contract for large results. Storage and MCP resource serving remain owned by
// their respective adapters.
package resultresource

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"strings"
	"time"
)

const (
	scheme       = "flashgate"
	resultHost   = "result"
	opaqueBytes  = 24
	SHA256Prefix = "sha256:"
)

var (
	ErrInvalidDescriptor = errors.New("invalid result resource descriptor")
	ErrAccessDenied      = errors.New("result resource access denied")
	ErrExpired           = errors.New("result resource expired")
)

// Binding contains the authorization and lifecycle context that must match on
// every access. Values come from trusted server context, never tool arguments.
type Binding struct {
	Principal         string
	Root              string
	Profile           string
	Capabilities      string
	ExecutionBackend  string
	ServiceGeneration string
	Operation         string
}

// Metadata describes content without exposing a host path.
type Metadata struct {
	MIMEType string
	Size     int64
	Digest   string
}

// Descriptor is the transport-neutral reference returned by a core domain.
type Descriptor struct {
	URI       string
	Metadata  Metadata
	Binding   Binding
	ExpiresAt time.Time
}

// New creates an unpredictable, expiring result reference. Content is passed
// only to derive safe metadata and is not retained by the descriptor.
func New(content []byte, mimeType string, binding Binding, expiresAt time.Time) (Descriptor, error) {
	if err := validateBinding(binding); err != nil {
		return Descriptor{}, err
	}
	if _, _, err := mime.ParseMediaType(mimeType); err != nil || !expiresAt.After(time.Now()) {
		return Descriptor{}, ErrInvalidDescriptor
	}

	id := make([]byte, opaqueBytes)
	if _, err := rand.Read(id); err != nil {
		return Descriptor{}, fmt.Errorf("generate result resource identifier: %w", err)
	}
	sum := sha256.Sum256(content)
	return Descriptor{
		URI:       scheme + "://" + resultHost + "/" + base64.RawURLEncoding.EncodeToString(id),
		Metadata:  Metadata{MIMEType: mimeType, Size: int64(len(content)), Digest: SHA256Prefix + hex.EncodeToString(sum[:])},
		Binding:   binding,
		ExpiresAt: expiresAt.UTC(),
	}, nil
}

// Authorize repeats the complete binding and expiry checks required before an
// adapter retrieves, pages, streams, or deletes the referenced content.
func (d Descriptor) Authorize(actual Binding, now time.Time) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if !now.Before(d.ExpiresAt) {
		return ErrExpired
	}
	if !bindingsEqual(d.Binding, actual) {
		return ErrAccessDenied
	}
	return nil
}

// Validate rejects malformed or path-bearing public references.
func (d Descriptor) Validate() error {
	u, err := url.Parse(d.URI)
	if err != nil || u.Scheme != scheme || u.Host != resultHost || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return ErrInvalidDescriptor
	}
	id, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(u.EscapedPath(), "/"))
	if err != nil || len(id) != opaqueBytes || strings.Count(u.Path, "/") != 1 {
		return ErrInvalidDescriptor
	}
	if _, _, err := mime.ParseMediaType(d.Metadata.MIMEType); err != nil {
		return ErrInvalidDescriptor
	}
	if err := validateBinding(d.Binding); err != nil || d.Metadata.Size < 0 || d.ExpiresAt.IsZero() {
		return ErrInvalidDescriptor
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(d.Metadata.Digest, SHA256Prefix))
	if err != nil || !strings.HasPrefix(d.Metadata.Digest, SHA256Prefix) || len(digest) != sha256.Size {
		return ErrInvalidDescriptor
	}
	return nil
}

func validateBinding(b Binding) error {
	if strings.TrimSpace(b.Principal) == "" || strings.TrimSpace(b.Root) == "" || strings.TrimSpace(b.Profile) == "" ||
		strings.TrimSpace(b.Capabilities) == "" || strings.TrimSpace(b.ExecutionBackend) == "" ||
		strings.TrimSpace(b.ServiceGeneration) == "" || strings.TrimSpace(b.Operation) == "" {
		return ErrInvalidDescriptor
	}
	return nil
}

func bindingsEqual(a, b Binding) bool {
	return equal(a.Principal, b.Principal) && equal(a.Root, b.Root) && equal(a.Profile, b.Profile) &&
		equal(a.Capabilities, b.Capabilities) && equal(a.ExecutionBackend, b.ExecutionBackend) &&
		equal(a.ServiceGeneration, b.ServiceGeneration) && equal(a.Operation, b.Operation)
}

func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
