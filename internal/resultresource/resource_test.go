package resultresource

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDescriptorLifecycle(t *testing.T) {
	now := time.Now().UTC()
	binding := testBinding()
	d, err := New([]byte("large result"), "text/plain; charset=utf-8", binding, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.URI, "flashgate://result/") || strings.Contains(d.URI, "large") {
		t.Fatalf("unsafe URI %q", d.URI)
	}
	if d.Metadata.Size != 12 || !strings.HasPrefix(d.Metadata.Digest, SHA256Prefix) {
		t.Fatalf("unexpected metadata: %#v", d.Metadata)
	}
	if err := d.Authorize(binding, now); err != nil {
		t.Fatalf("authorized access failed: %v", err)
	}

	other := binding
	other.Principal = "other"
	if err := d.Authorize(other, now); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("cross-principal access error = %v", err)
	}
	if err := d.Authorize(binding, d.ExpiresAt); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired access error = %v", err)
	}
}

func TestDescriptorRejectsMalformedReferencesAndBindings(t *testing.T) {
	binding := testBinding()
	if _, err := New(nil, "application/octet-stream", Binding{}, time.Now().Add(time.Minute)); !errors.Is(err, ErrInvalidDescriptor) {
		t.Fatalf("empty binding error = %v", err)
	}
	if _, err := New(nil, "not a media type", binding, time.Now().Add(time.Minute)); !errors.Is(err, ErrInvalidDescriptor) {
		t.Fatalf("invalid MIME type error = %v", err)
	}
	d, err := New(nil, "application/octet-stream", binding, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, uri := range []string{
		"file:///private/result", "flashgate://result/a/b", "flashgate://result/not-base64!", d.URI + "?path=/private",
	} {
		bad := d
		bad.URI = uri
		if err := bad.Validate(); !errors.Is(err, ErrInvalidDescriptor) {
			t.Errorf("Validate(%q) = %v", uri, err)
		}
	}
}

func testBinding() Binding {
	return Binding{
		Principal: "principal-1", Root: "root-1", Profile: "safe-read",
		Capabilities: "caps-sha256", ExecutionBackend: "direct",
		ServiceGeneration: "generation-1", Operation: "operation-1",
	}
}
