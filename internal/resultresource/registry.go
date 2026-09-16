// Package resultresource provides transport-neutral, identity-bound storage for
// payloads that are too large to return inline. Protocol adapters may project
// descriptors as resource links, but the registry does not depend on MCP.
package resultresource

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strings"
	"sync"
	"time"
)

const handlePrefix = "flashgate://result/"

var (
	ErrInvalidArgument      = errors.New("invalid result resource argument")
	ErrLimitExceeded        = errors.New("result resource limit exceeded")
	ErrUnavailable          = errors.New("result resource unavailable")
	ErrResourceLinkRequired = errors.New("resource link support required")
)

// Binding identifies the server-derived authorization context for a resource.
// Every field participates in access checks; callers must not populate it from
// untrusted tool arguments.
type Binding struct {
	Principal         string
	Profile           string
	Root              string
	ExecutionBackend  string
	ServiceGeneration string
}

func (b Binding) valid() bool {
	return b.Principal != "" && b.Profile != "" && b.Root != "" &&
		b.ExecutionBackend != "" && b.ServiceGeneration != ""
}

func (b Binding) equal(other Binding) bool {
	return constantTimeEqual(b.Principal, other.Principal) &&
		constantTimeEqual(b.Profile, other.Profile) &&
		constantTimeEqual(b.Root, other.Root) &&
		constantTimeEqual(b.ExecutionBackend, other.ExecutionBackend) &&
		constantTimeEqual(b.ServiceGeneration, other.ServiceGeneration)
}

func constantTimeEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

// Limits bounds both individual results and aggregate in-memory retention.
type Limits struct {
	MaxResourceBytes  int64
	MaxInlineBytes    int64
	MaxPageBytes      int64
	MaxResources      int
	MaxTotalBytes     int64
	MaxPrincipalBytes int64
	MaxTTL            time.Duration
}

func (l Limits) valid() bool {
	return l.MaxResourceBytes > 0 && l.MaxInlineBytes >= 0 &&
		l.MaxInlineBytes <= l.MaxResourceBytes && l.MaxPageBytes > 0 &&
		l.MaxPageBytes <= l.MaxResourceBytes && l.MaxResources > 0 &&
		l.MaxTotalBytes >= l.MaxResourceBytes &&
		l.MaxPrincipalBytes >= l.MaxResourceBytes && l.MaxTTL > 0
}

// Descriptor contains only safe, portable metadata. URI is opaque and never
// contains a host path or caller identity.
type Descriptor struct {
	URI       string
	MediaType string
	Size      int64
	SHA256    string
	ExpiresAt time.Time
}

// Page is a bounded resource read. NextOffset is meaningful only when EOF is
// false.
type Page struct {
	Data       []byte
	NextOffset int64
	EOF        bool
}

// DeliveryMode tells a protocol adapter whether it may inline the payload or
// must emit a negotiated resource link.
type DeliveryMode string

const (
	DeliveryInline       DeliveryMode = "inline"
	DeliveryResourceLink DeliveryMode = "resource_link"
)

type entry struct {
	descriptor Descriptor
	binding    Binding
	content    []byte
}

// Registry owns bounded retained result payloads. It is safe for concurrent
// use. Expired entries are removed during storage and reads and may also be
// removed proactively with SweepExpired.
type Registry struct {
	mu             sync.Mutex
	limits         Limits
	now            func() time.Time
	random         io.Reader
	entries        map[string]entry
	totalBytes     int64
	principalBytes map[string]int64
}

// New constructs a registry. Passing nil for random uses crypto/rand.Reader;
// passing nil for now uses time.Now.
func New(limits Limits, random io.Reader, now func() time.Time) (*Registry, error) {
	if !limits.valid() {
		return nil, ErrInvalidArgument
	}
	if random == nil {
		random = rand.Reader
	}
	if now == nil {
		now = time.Now
	}
	return &Registry{
		limits:         limits,
		now:            now,
		random:         random,
		entries:        make(map[string]entry),
		principalBytes: make(map[string]int64),
	}, nil
}

// Store retains an immutable copy of content and returns its safe descriptor.
func (r *Registry) Store(binding Binding, mediaType string, content []byte, ttl time.Duration) (Descriptor, error) {
	if !binding.valid() || ttl <= 0 || ttl > r.limits.MaxTTL || len(content) == 0 || int64(len(content)) > r.limits.MaxResourceBytes {
		return Descriptor{}, ErrInvalidArgument
	}
	parsedMediaType, _, err := mime.ParseMediaType(mediaType)
	if err != nil || !strings.Contains(parsedMediaType, "/") {
		return Descriptor{}, ErrInvalidArgument
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked(r.now())
	size := int64(len(content))
	if len(r.entries) >= r.limits.MaxResources || r.totalBytes+size > r.limits.MaxTotalBytes ||
		r.principalBytes[binding.Principal]+size > r.limits.MaxPrincipalBytes {
		return Descriptor{}, ErrLimitExceeded
	}

	uri, err := r.newURILocked()
	if err != nil {
		return Descriptor{}, fmt.Errorf("generate result resource handle: %w", err)
	}
	digest := sha256.Sum256(content)
	descriptor := Descriptor{
		URI:       uri,
		MediaType: parsedMediaType,
		Size:      size,
		SHA256:    fmt.Sprintf("%x", digest),
		ExpiresAt: r.now().Add(ttl).UTC(),
	}
	r.entries[uri] = entry{descriptor: descriptor, binding: binding, content: append([]byte(nil), content...)}
	r.totalBytes += size
	r.principalBytes[binding.Principal] += size
	return descriptor, nil
}

func (r *Registry) newURILocked() (string, error) {
	for range 4 {
		buffer := make([]byte, 32)
		if _, err := io.ReadFull(r.random, buffer); err != nil {
			return "", err
		}
		uri := handlePrefix + base64.RawURLEncoding.EncodeToString(buffer)
		if _, exists := r.entries[uri]; !exists {
			return uri, nil
		}
	}
	return "", errors.New("could not allocate a unique handle")
}

// Read returns a copy of one bounded page. Unknown, expired, malformed, and
// cross-owner handles deliberately have the same result.
func (r *Registry) Read(binding Binding, uri string, offset, limit int64) (Page, error) {
	if !binding.valid() || offset < 0 || limit <= 0 || limit > r.limits.MaxPageBytes || !validURI(uri) {
		return Page{}, ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	r.sweepLocked(now)
	stored, ok := r.entries[uri]
	if !ok || !stored.binding.equal(binding) || offset > int64(len(stored.content)) {
		return Page{}, ErrUnavailable
	}
	end := offset + limit
	if end > int64(len(stored.content)) {
		end = int64(len(stored.content))
	}
	return Page{
		Data:       append([]byte(nil), stored.content[offset:end]...),
		NextOffset: end,
		EOF:        end == int64(len(stored.content)),
	}, nil
}

// SelectDelivery applies the configured inline ceiling and negotiated-link
// state without coupling the core registry to a protocol-specific DTO.
func (r *Registry) SelectDelivery(size int64, resourceLinksNegotiated bool) (DeliveryMode, error) {
	if size < 0 || size > r.limits.MaxResourceBytes {
		return "", ErrInvalidArgument
	}
	if size <= r.limits.MaxInlineBytes {
		return DeliveryInline, nil
	}
	if !resourceLinksNegotiated {
		return "", ErrResourceLinkRequired
	}
	return DeliveryResourceLink, nil
}

// SweepExpired removes expired entries and returns the number removed.
func (r *Registry) SweepExpired() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sweepLocked(r.now())
}

func (r *Registry) sweepLocked(now time.Time) int {
	removed := 0
	for uri, stored := range r.entries {
		if now.Before(stored.descriptor.ExpiresAt) {
			continue
		}
		delete(r.entries, uri)
		r.totalBytes -= stored.descriptor.Size
		r.principalBytes[stored.binding.Principal] -= stored.descriptor.Size
		if r.principalBytes[stored.binding.Principal] == 0 {
			delete(r.principalBytes, stored.binding.Principal)
		}
		removed++
	}
	return removed
}

func validURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "flashgate" || parsed.Host != "result" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	token := strings.TrimPrefix(parsed.EscapedPath(), "/")
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32 && raw == handlePrefix+token
}
