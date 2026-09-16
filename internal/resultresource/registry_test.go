package resultresource

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func testLimits() Limits {
	return Limits{
		MaxResourceBytes:  16,
		MaxInlineBytes:    4,
		MaxPageBytes:      5,
		MaxResources:      3,
		MaxTotalBytes:     24,
		MaxPrincipalBytes: 16,
		MaxTTL:            time.Minute,
	}
}

func testBinding(principal string) Binding {
	return Binding{Principal: principal, Profile: "read", Root: "docs", ExecutionBackend: "direct", ServiceGeneration: "generation-1"}
}

func TestRegistryStoreReadAndDescriptor(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{0x42}, 64))
	registry, err := New(testLimits(), random, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := registry.Store(testBinding("alice"), "text/plain; charset=utf-8", []byte("abcdefgh"), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(descriptor.URI, handlePrefix) || strings.Contains(descriptor.URI, "alice") || strings.Contains(descriptor.URI, "docs") {
		t.Fatalf("handle is not opaque: %q", descriptor.URI)
	}
	if descriptor.MediaType != "text/plain" || descriptor.Size != 8 || descriptor.SHA256 != "9c56cc51b374c3ba189210d5b6d4bf57790d351c96c47c02190ecf1e430635ab" {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	if !descriptor.ExpiresAt.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("expires at %v", descriptor.ExpiresAt)
	}

	page, err := registry.Read(testBinding("alice"), descriptor.URI, 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(page.Data) != "cdefg" || page.NextOffset != 7 || page.EOF {
		t.Fatalf("unexpected page: %#v", page)
	}
	page.Data[0] = 'X'
	again, err := registry.Read(testBinding("alice"), descriptor.URI, 2, 5)
	if err != nil || string(again.Data) != "cdefg" {
		t.Fatalf("stored content aliased caller data: %#v, %v", again, err)
	}
}

func TestRegistryOwnerAndExpiryAreIndistinguishable(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	registry, err := New(testLimits(), bytes.NewReader(bytes.Repeat([]byte{1}, 64)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := registry.Store(testBinding("alice"), "application/octet-stream", []byte("secret"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		binding Binding
		uri     string
	}{
		"other owner": {testBinding("bob"), descriptor.URI},
		"unknown":     {testBinding("alice"), handlePrefix + strings.Repeat("A", 43)},
		"malformed":   {testBinding("alice"), "file:///secret"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := registry.Read(tc.binding, tc.uri, 0, 1)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("got %v", err)
			}
		})
	}
	now = now.Add(time.Second)
	if removed := registry.SweepExpired(); removed != 1 {
		t.Fatalf("removed %d entries", removed)
	}
	if _, err := registry.Read(testBinding("alice"), descriptor.URI, 0, 1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expired read: %v", err)
	}
}

func TestRegistryLimitsAndExpiredCapacityReuse(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte("unique-handle-material-0123456789"), 8))
	registry, err := New(testLimits(), random, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Store(testBinding("alice"), "text/plain", make([]byte, 17), time.Second); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("oversized resource: %v", err)
	}
	if _, err := registry.Store(testBinding("alice"), "not a media type", []byte("x"), time.Second); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid media type: %v", err)
	}
	if _, err := registry.Store(testBinding("alice"), "text/plain", make([]byte, 16), time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Store(testBinding("alice"), "text/plain", []byte("x"), time.Second); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("principal limit: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := registry.Store(testBinding("alice"), "text/plain", []byte("replacement"), time.Second); err != nil {
		t.Fatalf("expired capacity was not reused: %v", err)
	}
}

func TestSelectDelivery(t *testing.T) {
	registry, err := New(testLimits(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mode, err := registry.SelectDelivery(4, false); err != nil || mode != DeliveryInline {
		t.Fatalf("inline: %q, %v", mode, err)
	}
	if _, err := registry.SelectDelivery(5, false); !errors.Is(err, ErrResourceLinkRequired) {
		t.Fatalf("fallback: %v", err)
	}
	if mode, err := registry.SelectDelivery(5, true); err != nil || mode != DeliveryResourceLink {
		t.Fatalf("resource link: %q, %v", mode, err)
	}
}

func TestNewRejectsUnsafeLimits(t *testing.T) {
	limits := testLimits()
	limits.MaxPageBytes = limits.MaxResourceBytes + 1
	if _, err := New(limits, nil, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("got %v", err)
	}
}
