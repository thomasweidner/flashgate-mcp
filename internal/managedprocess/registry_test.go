package managedprocess

import (
	"errors"
	"math"
	"sync"
	"testing"
)

type testProcess struct {
	name string
}

func TestRegistryLifecycleAndIdentifierNonReuse(t *testing.T) {
	registry := NewRegistry[*testProcess]()
	first := &testProcess{name: "first"}
	second := &testProcess{name: "second"}

	firstID, err := registry.Register(first)
	if err != nil {
		t.Fatalf("register first process: %v", err)
	}
	secondID, err := registry.Register(second)
	if err != nil {
		t.Fatalf("register second process: %v", err)
	}
	if firstID == 0 || secondID == 0 || firstID == secondID {
		t.Fatalf("expected distinct non-zero instance IDs, got %d and %d", firstID, secondID)
	}
	if registry.Len() != 2 {
		t.Fatalf("expected two registered processes, got %d", registry.Len())
	}

	got, ok := registry.Get(firstID)
	if !ok || got != first {
		t.Fatalf("Get(%d) = (%v, %t), want (%v, true)", firstID, got, ok, first)
	}
	removed, ok := registry.Remove(firstID)
	if !ok || removed != first {
		t.Fatalf("Remove(%d) = (%v, %t), want (%v, true)", firstID, removed, ok, first)
	}
	if _, ok := registry.Get(firstID); ok {
		t.Fatalf("removed instance ID %d remained registered", firstID)
	}
	if _, ok := registry.Remove(firstID); ok {
		t.Fatalf("second removal of instance ID %d unexpectedly succeeded", firstID)
	}

	thirdID, err := registry.Register(&testProcess{name: "third"})
	if err != nil {
		t.Fatalf("register third process: %v", err)
	}
	if thirdID == firstID {
		t.Fatalf("removed instance ID %d was reused", firstID)
	}
}

func TestRegistryZeroValueIsUsable(t *testing.T) {
	var registry Registry[string]

	id, err := registry.Register("process")
	if err != nil {
		t.Fatalf("register with zero-value registry: %v", err)
	}
	if got, ok := registry.Get(id); !ok || got != "process" {
		t.Fatalf("Get(%d) = (%q, %t), want (%q, true)", id, got, ok, "process")
	}
}

func TestRegistryRejectsIdentifierWraparound(t *testing.T) {
	registry := NewRegistry[string]()
	registry.lastID = InstanceID(math.MaxUint64)

	id, err := registry.Register("process")
	if id != 0 || !errors.Is(err, ErrIdentifierExhausted) {
		t.Fatalf("Register() = (%d, %v), want (0, ErrIdentifierExhausted)", id, err)
	}
	if registry.Len() != 0 {
		t.Fatalf("exhausted registry mutated, length = %d", registry.Len())
	}
}

func TestRegistryConcurrentLifecycle(t *testing.T) {
	const processCount = 256

	registry := NewRegistry[*testProcess]()
	ids := make(chan InstanceID, processCount)
	var registerGroup sync.WaitGroup

	for i := 0; i < processCount; i++ {
		registerGroup.Add(1)
		go func() {
			defer registerGroup.Done()
			id, err := registry.Register(&testProcess{name: "managed"})
			if err != nil {
				t.Errorf("Register() error = %v", err)
				return
			}
			ids <- id
		}()
	}
	registerGroup.Wait()
	close(ids)

	seen := make(map[InstanceID]struct{}, processCount)
	for id := range ids {
		if id == 0 {
			t.Fatal("Register() returned zero instance ID")
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate instance ID %d", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != processCount || registry.Len() != processCount {
		t.Fatalf("registered %d unique processes and registry length %d, want %d", len(seen), registry.Len(), processCount)
	}

	var removeGroup sync.WaitGroup
	for id := range seen {
		removeGroup.Add(1)
		go func(id InstanceID) {
			defer removeGroup.Done()
			if _, ok := registry.Remove(id); !ok {
				t.Errorf("Remove(%d) did not find registered process", id)
			}
		}(id)
	}
	removeGroup.Wait()
	if registry.Len() != 0 {
		t.Fatalf("registry length after concurrent removal = %d, want 0", registry.Len())
	}
}
