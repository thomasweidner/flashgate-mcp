package operation

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateHandle(t *testing.T) {
	binding := validHandleBinding()
	handle, err := generateHandle(binding, strings.NewReader(strings.Repeat("a", handleEntropyBytes)))
	if err != nil {
		t.Fatalf("generate handle: %v", err)
	}

	if !regexp.MustCompile(`^op_[A-Za-z0-9_-]{32}$`).MatchString(handle.ID()) {
		t.Fatalf("unexpected opaque handle format: %q", handle.ID())
	}
	if strings.Contains(handle.ID(), binding.PrincipalID) || strings.Contains(handle.ID(), binding.RootID) {
		t.Fatalf("handle ID exposes ownership context: %q", handle.ID())
	}
	if got := handle.Binding(); got != binding {
		t.Fatalf("binding = %#v, want %#v", got, binding)
	}
}

func TestGenerateHandleRejectsIncompleteOrNonCanonicalBinding(t *testing.T) {
	tests := map[string]func(*HandleBinding){
		"principal":          func(b *HandleBinding) { b.PrincipalID = "" },
		"root":               func(b *HandleBinding) { b.RootID = "" },
		"profile":            func(b *HandleBinding) { b.ProfileID = "" },
		"execution backend":  func(b *HandleBinding) { b.ExecutionBackend = "" },
		"service generation": func(b *HandleBinding) { b.ServiceGeneration = "" },
		"surrounding space":  func(b *HandleBinding) { b.RootID = " root-a" },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			binding := validHandleBinding()
			mutate(&binding)
			handle, err := generateHandle(binding, strings.NewReader(strings.Repeat("a", handleEntropyBytes)))
			if !errors.Is(err, ErrInvalidHandleBinding) {
				t.Fatalf("error = %v, want ErrInvalidHandleBinding", err)
			}
			if handle != (Handle{}) {
				t.Fatalf("handle = %#v, want zero value", handle)
			}
		})
	}
}

func TestGenerateHandleFailsClosedWhenEntropyUnavailable(t *testing.T) {
	handle, err := generateHandle(validHandleBinding(), strings.NewReader("short"))
	if !errors.Is(err, ErrHandleGeneration) {
		t.Fatalf("error = %v, want ErrHandleGeneration", err)
	}
	if handle != (Handle{}) {
		t.Fatalf("handle = %#v, want zero value", handle)
	}
}

func TestGenerateHandleProducesUniqueIDs(t *testing.T) {
	const count = 256
	seen := make(map[string]struct{}, count)
	for range count {
		handle, err := GenerateHandle(validHandleBinding())
		if err != nil {
			t.Fatalf("generate handle: %v", err)
		}
		if _, exists := seen[handle.ID()]; exists {
			t.Fatalf("duplicate handle ID: %q", handle.ID())
		}
		seen[handle.ID()] = struct{}{}
	}
}

func TestHandleBindingReturnsCopy(t *testing.T) {
	handle, err := GenerateHandle(validHandleBinding())
	if err != nil {
		t.Fatalf("generate handle: %v", err)
	}

	copy := handle.Binding()
	copy.PrincipalID = "other-principal"
	if handle.Binding().PrincipalID != "principal-a" {
		t.Fatal("mutating returned binding changed handle ownership")
	}
}

func validHandleBinding() HandleBinding {
	return HandleBinding{
		PrincipalID:       "principal-a",
		RootID:            "root-a",
		ProfileID:         "profile-a",
		ExecutionBackend:  "service-account",
		ServiceGeneration: "generation-a",
	}
}
