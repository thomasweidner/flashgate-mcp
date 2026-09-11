package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePathAppliesPolicyWithoutMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := filesystem.ValidatePath("existing.txt", true); err != nil {
		t.Fatalf("existing path validation failed: %v", err)
	}
	if err := filesystem.ValidatePath("new.txt", false); err != nil {
		t.Fatalf("create path validation failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("ValidatePath mutated create target: %v", err)
	}
	if err := filesystem.ValidatePath("../outside.txt", false); err == nil {
		t.Fatal("expected traversal validation error")
	}
	if err := filesystem.ValidatePath("missing.txt", true); err == nil {
		t.Fatal("expected missing existing path validation error")
	}
}
