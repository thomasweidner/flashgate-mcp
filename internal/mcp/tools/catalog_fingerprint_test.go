package tools

import (
	"strings"
	"testing"
)

func TestCatalogFingerprintIsDeterministicAndContextBound(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.Register(&testTool{name: "read_file", description: "Reads a file.", inputSchema: map[string]any{"type": "object"}})

	context := CatalogContext{
		ProtocolRevision: "2025-11-25", Extensions: []string{"example/b", "example/a"},
		Profile: "safe-read", Capabilities: []string{"filesystem.read", "filesystem.list"},
		RiskPolicyVersion: "policy-v1", SchemaVersion: "schema-v1", ConfigurationGeneration: "config-7",
	}
	fingerprint, err := registry.CatalogFingerprint(context)
	if err != nil {
		t.Fatalf("CatalogFingerprint() error = %v", err)
	}
	if !strings.HasPrefix(fingerprint, "sha256:") || len(fingerprint) != len("sha256:")+64 {
		t.Fatalf("CatalogFingerprint() = %q, want sha256 digest", fingerprint)
	}

	reordered := context
	reordered.Extensions = []string{"example/a", "example/b"}
	reordered.Capabilities = []string{"filesystem.list", "filesystem.read"}
	got, err := registry.CatalogFingerprint(reordered)
	if err != nil {
		t.Fatalf("CatalogFingerprint(reordered) error = %v", err)
	}
	if got != fingerprint {
		t.Fatalf("set ordering changed fingerprint: %q != %q", got, fingerprint)
	}

	changed := context
	changed.ConfigurationGeneration = "config-8"
	got, err = registry.CatalogFingerprint(changed)
	if err != nil {
		t.Fatalf("CatalogFingerprint(changed) error = %v", err)
	}
	if got == fingerprint {
		t.Fatal("configuration generation did not change fingerprint")
	}
}

func TestCatalogFingerprintChangesWithDefinitionAndRegistrationOrder(t *testing.T) {
	t.Parallel()

	context := CatalogContext{
		ProtocolRevision: "2025-11-25", Profile: "safe-read", RiskPolicyVersion: "policy-v1",
		SchemaVersion: "schema-v1", ConfigurationGeneration: "config-1",
	}
	newTool := func(name, description string) *testTool {
		return &testTool{name: name, description: description, inputSchema: map[string]any{"type": "object"}}
	}

	first := NewRegistry()
	first.Register(newTool("a", "A"))
	first.Register(newTool("b", "B"))
	baseline, err := first.CatalogFingerprint(context)
	if err != nil {
		t.Fatalf("CatalogFingerprint(first) error = %v", err)
	}

	changed := NewRegistry()
	changed.Register(newTool("a", "changed"))
	changed.Register(newTool("b", "B"))
	changedFingerprint, err := changed.CatalogFingerprint(context)
	if err != nil {
		t.Fatalf("CatalogFingerprint(changed) error = %v", err)
	}
	if changedFingerprint == baseline {
		t.Fatal("tool definition did not change fingerprint")
	}

	reordered := NewRegistry()
	reordered.Register(newTool("b", "B"))
	reordered.Register(newTool("a", "A"))
	reorderedFingerprint, err := reordered.CatalogFingerprint(context)
	if err != nil {
		t.Fatalf("CatalogFingerprint(reordered) error = %v", err)
	}
	if reorderedFingerprint == baseline {
		t.Fatal("registration order did not change fingerprint")
	}
}

func TestCatalogFingerprintRejectsIncompleteOrAmbiguousContext(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	valid := CatalogContext{
		ProtocolRevision: "2025-11-25", Profile: "safe-read", RiskPolicyVersion: "policy-v1",
		SchemaVersion: "schema-v1", ConfigurationGeneration: "config-1",
	}

	if _, err := registry.CatalogFingerprint(CatalogContext{}); err == nil {
		t.Fatal("CatalogFingerprint(incomplete) expected error")
	}
	duplicate := valid
	duplicate.Capabilities = []string{"filesystem.read", "filesystem.read"}
	if _, err := registry.CatalogFingerprint(duplicate); err == nil {
		t.Fatal("CatalogFingerprint(duplicate capabilities) expected error")
	}
	empty := valid
	empty.Extensions = []string{""}
	if _, err := registry.CatalogFingerprint(empty); err == nil {
		t.Fatal("CatalogFingerprint(empty extension) expected error")
	}

	unserializable := NewRegistry()
	unserializable.Register(&testTool{name: "bad", description: "bad", inputSchema: make(chan int)})
	if _, err := unserializable.CatalogFingerprint(valid); err == nil {
		t.Fatal("CatalogFingerprint(non-serializable definition) expected error")
	}
}
