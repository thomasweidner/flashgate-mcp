//go:build windows

package security

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestResolveExistingRejectsWindowsHiddenAttributeByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "hidden.txt")
	writeSecurityTestFile(t, path, "hidden")
	setHiddenAttributeOrSkip(t, path)

	guard := mustNewPathGuard(t, root)

	_, err := guard.ResolveExisting("hidden.txt")

	if !errors.Is(err, ErrHiddenPathDenied) {
		t.Fatalf("expected ErrHiddenPathDenied, got %v", err)
	}
}

func TestResolveRejectsWindowsRootedAndVolumeRelativeOperands(t *testing.T) {
	guard, err := NewPathGuardWithPolicy(t.TempDir(), Policy{AllowUNCPaths: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{`C:relative.txt`, `c:\absolute.txt`, `\rooted.txt`, `\\server\share\file.txt`, `\\?\C:\absolute.txt`} {
		if _, err := guard.ResolveForCreate(path); !errors.Is(err, ErrAbsolutePath) {
			t.Errorf("expected absolute-path denial for %q, got %v", path, err)
		}
	}
}

func TestResolveExistingAllowsWindowsHiddenAttributeWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "hidden.txt")
	writeSecurityTestFile(t, path, "hidden")
	setHiddenAttributeOrSkip(t, path)

	guard := mustNewPathGuardWithPolicy(t, root, Policy{AllowHiddenFiles: true})

	_, err := guard.ResolveExisting("hidden.txt")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestNewPathGuardAcceptsWindowsRootCaseVariant(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	caseVariant := strings.ToUpper(root)
	guard, err := NewPathGuard(caseVariant)
	if err != nil {
		t.Fatalf("expected case-variant root to be accepted, got %v", err)
	}
	if !strings.EqualFold(guard.Root(), filepath.Clean(root)) {
		t.Fatalf("expected equivalent Windows root, got %q and %q", guard.Root(), root)
	}
}

func TestNewPathGuardRejectsWindowsJunctionRoot(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	junction := filepath.Join(t.TempDir(), "junction-root")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("junction creation is not available: %v (%s)", err, output)
	}

	_, err := NewPathGuardWithPolicy(junction, Policy{FollowSymlinks: true})
	if !errors.Is(err, ErrReparsePointDenied) {
		t.Fatalf("expected ErrReparsePointDenied, got %v", err)
	}
}

func setHiddenAttributeOrSkip(t *testing.T, path string) {
	t.Helper()

	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("failed to encode path: %v", err)
	}

	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		t.Skipf("failed to read file attributes: %v", err)
	}

	if err := syscall.SetFileAttributes(pointer, attributes|fileAttributeHidden); err != nil {
		t.Skipf("failed to set hidden attribute: %v", err)
	}
}
