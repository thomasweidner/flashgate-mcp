package operation

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestRegistryLifecycleAndOwnership(t *testing.T) {
	registry := NewRegistry[string]()
	record := Record[string]{ID: "internal-1", Domain: "filesystem", Value: "queued work"}

	if err := registry.Register(record); err != nil {
		t.Fatalf("register: %v", err)
	}
	if registry.Len() != 1 {
		t.Fatalf("len=%d, want 1", registry.Len())
	}
	if err := registry.Register(record); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("duplicate register error=%v, want ErrAlreadyRegistered", err)
	}
	if _, err := registry.Get("search", record.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get error=%v, want ErrNotFound", err)
	}
	if err := registry.Replace("search", record.ID, "stolen"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner replace error=%v, want ErrNotFound", err)
	}

	if err := registry.Replace(record.Domain, record.ID, "running work"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := registry.Get(record.Domain, record.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != record.ID || got.Domain != record.Domain || got.Value != "running work" {
		t.Fatalf("unexpected record: %#v", got)
	}

	removed, err := registry.Remove(record.Domain, record.ID)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removed.Value != "running work" || registry.Len() != 0 {
		t.Fatalf("unexpected removed record or length: %#v len=%d", removed, registry.Len())
	}
	if _, err := registry.Get(record.Domain, record.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after remove error=%v, want ErrNotFound", err)
	}
}

func TestRegistryRejectsIncompleteIdentity(t *testing.T) {
	registry := NewRegistry[int]()
	for _, record := range []Record[int]{{Domain: "filesystem"}, {ID: "internal-1"}, {}} {
		if err := registry.Register(record); !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("register %#v error=%v, want ErrInvalidIdentity", record, err)
		}
	}
	if registry.Len() != 0 {
		t.Fatalf("len=%d, want 0", registry.Len())
	}
}

func TestRegistryConcurrentIndependentLifecycles(t *testing.T) {
	const count = 128
	registry := NewRegistry[int]()
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("internal-%d", i)
			if err := registry.Register(Record[int]{ID: id, Domain: "filesystem", Value: i}); err != nil {
				t.Errorf("register %s: %v", id, err)
				return
			}
			if err := registry.Replace("filesystem", id, i+1); err != nil {
				t.Errorf("replace %s: %v", id, err)
				return
			}
			if _, err := registry.Remove("filesystem", id); err != nil {
				t.Errorf("remove %s: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	if registry.Len() != 0 {
		t.Fatalf("len=%d, want 0", registry.Len())
	}
}
