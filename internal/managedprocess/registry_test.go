package managedprocess

import (
	"bytes"
	"encoding/base64"
	"errors"
	"math"
	"sync"
	"testing"
)

type testProcess struct {
	name string
}

func TestRegistryLifecycleAndHandleNonReuse(t *testing.T) {
	registry := NewRegistry[*testProcess]()
	owner := PrincipalID("connection-a")
	first := &testProcess{name: "first"}
	second := &testProcess{name: "second"}

	firstHandle, err := registry.Register(owner, first)
	if err != nil {
		t.Fatalf("register first process: %v", err)
	}
	secondHandle, err := registry.Register(owner, second)
	if err != nil {
		t.Fatalf("register second process: %v", err)
	}
	if firstHandle == "" || secondHandle == "" || firstHandle == secondHandle {
		t.Fatalf("expected distinct non-empty handles, got %q and %q", firstHandle, secondHandle)
	}
	if registry.Len() != 2 {
		t.Fatalf("expected two registered processes, got %d", registry.Len())
	}

	got, ok := registry.Get(owner, firstHandle)
	if !ok || got != first {
		t.Fatalf("Get(%q) = (%v, %t), want (%v, true)", firstHandle, got, ok, first)
	}
	removed, ok := registry.Remove(owner, firstHandle)
	if !ok || removed != first {
		t.Fatalf("Remove(%q) = (%v, %t), want (%v, true)", firstHandle, removed, ok, first)
	}
	if _, ok := registry.Get(owner, firstHandle); ok {
		t.Fatalf("removed handle %q remained registered", firstHandle)
	}
	if _, ok := registry.Remove(owner, firstHandle); ok {
		t.Fatalf("second removal of handle %q unexpectedly succeeded", firstHandle)
	}

	thirdHandle, err := registry.Register(owner, &testProcess{name: "third"})
	if err != nil {
		t.Fatalf("register third process: %v", err)
	}
	if thirdHandle == firstHandle {
		t.Fatalf("removed handle %q was reused", firstHandle)
	}
}

func TestRegistryHandlesAreOpaqueAndCarryFullEntropy(t *testing.T) {
	registry := NewRegistry[string]()
	entropy := bytes.Repeat([]byte{0xa5}, handleEntropyBytes)
	registry.randomSource = bytes.NewReader(entropy)
	handle, err := registry.Register("owner", "process")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(string(handle))
	if err != nil {
		t.Fatalf("handle %q is not unpadded URL-safe base64: %v", handle, err)
	}
	if len(decoded) != handleEntropyBytes {
		t.Fatalf("decoded handle has %d bytes, want %d", len(decoded), handleEntropyBytes)
	}
	if !bytes.Equal(decoded, entropy) {
		t.Fatalf("decoded handle = %x, want random input %x", decoded, entropy)
	}
}

func TestRegistryEnforcesPrincipalBindingWithoutDisclosure(t *testing.T) {
	registry := NewRegistry[*testProcess]()
	process := &testProcess{name: "owned"}
	handle, err := registry.Register("owner-a", process)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if got, ok := registry.Get("owner-b", handle); ok || got != nil {
		t.Fatalf("wrong-principal Get() = (%v, %t), want (nil, false)", got, ok)
	}
	if got, ok := registry.Remove("owner-b", handle); ok || got != nil {
		t.Fatalf("wrong-principal Remove() = (%v, %t), want (nil, false)", got, ok)
	}
	if got, ok := registry.Get("owner-a", Handle("unknown")); ok || got != nil {
		t.Fatalf("unknown-handle Get() = (%v, %t), want (nil, false)", got, ok)
	}
	if got, ok := registry.Get("owner-a", handle); !ok || got != process {
		t.Fatalf("owner Get() = (%v, %t), want (%v, true)", got, ok, process)
	}
}

func TestRegistryRejectsEmptyPrincipal(t *testing.T) {
	registry := NewRegistry[string]()
	handle, err := registry.Register("", "process")
	if handle != "" || !errors.Is(err, ErrInvalidPrincipal) {
		t.Fatalf("Register() = (%q, %v), want empty handle and ErrInvalidPrincipal", handle, err)
	}
	if registry.Len() != 0 {
		t.Fatalf("invalid registration mutated registry, length = %d", registry.Len())
	}
}

func TestRegistryZeroValueIsUsable(t *testing.T) {
	var registry Registry[string]

	handle, err := registry.Register("owner", "process")
	if err != nil {
		t.Fatalf("register with zero-value registry: %v", err)
	}
	if got, ok := registry.Get("owner", handle); !ok || got != "process" {
		t.Fatalf("Get(%q) = (%q, %t), want (%q, true)", handle, got, ok, "process")
	}
}

func TestRegistryRejectsIdentifierWraparound(t *testing.T) {
	registry := NewRegistry[string]()
	registry.lastID = InstanceID(math.MaxUint64)

	handle, err := registry.Register("owner", "process")
	if handle != "" || !errors.Is(err, ErrIdentifierExhausted) {
		t.Fatalf("Register() = (%q, %v), want empty handle and ErrIdentifierExhausted", handle, err)
	}
	if registry.Len() != 0 {
		t.Fatalf("exhausted registry mutated, length = %d", registry.Len())
	}
}

func TestRegistryHandleGenerationFailureDoesNotMutate(t *testing.T) {
	registry := NewRegistry[string]()
	registry.randomSource = bytes.NewReader(nil)

	handle, err := registry.Register("owner", "process")
	if handle != "" || !errors.Is(err, ErrHandleUnavailable) {
		t.Fatalf("Register() = (%q, %v), want empty handle and ErrHandleUnavailable", handle, err)
	}
	if registry.Len() != 0 || registry.lastID != 0 {
		t.Fatalf("failed generation mutated registry: length=%d lastID=%d", registry.Len(), registry.lastID)
	}
}

func TestRegistryRetriesHandleCollisionWithoutAliasing(t *testing.T) {
	registry := NewRegistry[string]()
	firstEntropy := bytes.Repeat([]byte{0x11}, handleEntropyBytes)
	secondEntropy := bytes.Repeat([]byte{0x22}, handleEntropyBytes)
	registry.randomSource = bytes.NewReader(append(append(firstEntropy, firstEntropy...), secondEntropy...))

	firstHandle, err := registry.Register("owner", "first")
	if err != nil {
		t.Fatalf("register first process: %v", err)
	}
	secondHandle, err := registry.Register("owner", "second")
	if err != nil {
		t.Fatalf("register second process after collision: %v", err)
	}
	if firstHandle == secondHandle {
		t.Fatalf("colliding handle %q was reused", firstHandle)
	}
	if got, ok := registry.Get("owner", firstHandle); !ok || got != "first" {
		t.Fatalf("first handle changed after collision: (%q, %t)", got, ok)
	}
	if got, ok := registry.Get("owner", secondHandle); !ok || got != "second" {
		t.Fatalf("second handle lookup = (%q, %t), want (second, true)", got, ok)
	}
}

func TestRegistryConcurrentLifecycle(t *testing.T) {
	const processCount = 256

	registry := NewRegistry[*testProcess]()
	handles := make(chan Handle, processCount)
	var registerGroup sync.WaitGroup

	for i := 0; i < processCount; i++ {
		registerGroup.Add(1)
		go func() {
			defer registerGroup.Done()
			handle, err := registry.Register("owner", &testProcess{name: "managed"})
			if err != nil {
				t.Errorf("Register() error = %v", err)
				return
			}
			handles <- handle
		}()
	}
	registerGroup.Wait()
	close(handles)

	seen := make(map[Handle]struct{}, processCount)
	for handle := range handles {
		if handle == "" {
			t.Fatal("Register() returned empty handle")
		}
		if _, exists := seen[handle]; exists {
			t.Fatalf("duplicate handle %q", handle)
		}
		seen[handle] = struct{}{}
	}
	if len(seen) != processCount || registry.Len() != processCount {
		t.Fatalf("registered %d unique processes and registry length %d, want %d", len(seen), registry.Len(), processCount)
	}

	var removeGroup sync.WaitGroup
	for handle := range seen {
		removeGroup.Add(1)
		go func(handle Handle) {
			defer removeGroup.Done()
			if _, ok := registry.Remove("owner", handle); !ok {
				t.Errorf("Remove(%q) did not find registered process", handle)
			}
		}(handle)
	}
	removeGroup.Wait()
	if registry.Len() != 0 {
		t.Fatalf("registry length after concurrent removal = %d, want 0", registry.Len())
	}
}
