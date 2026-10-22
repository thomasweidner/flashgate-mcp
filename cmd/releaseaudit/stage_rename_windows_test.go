//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestStageRenameWindowsCodes(t *testing.T) {
	for _, code := range []syscall.Errno{5, 32, 2, 3, 80, 183, 87} {
		t.Run(fmt.Sprint(uintptr(code)), func(t *testing.T) {
			b, verify := stageTestBinding(t)
			injected := &os.LinkError{Op: "rename", Old: b.stage, New: b.target, Err: fmt.Errorf("wrapped: %w", code)}
			ops, _, sleeps := stageTestOps(nil)
			ops.transient = transientStageRename
			calls := 0
			ops.rename = func(string, string) error { calls++; return injected }
			err := b.renameWithOps(verify, ops)
			expected := 1
			if code == 5 || code == 32 {
				expected = 8
			}
			if calls != expected || *sleeps != expected-1 || !errors.Is(err, code) || !strings.Contains(err.Error(), fmt.Sprintf("Win32=%d", code)) {
				t.Fatalf("calls=%d sleeps=%d: %v", calls, *sleeps, err)
			}
		})
	}
	if transientStageRename(errors.New("Access is denied. ERROR_SHARING_VIOLATION 32")) {
		t.Fatal("message classified as errno")
	}
	defaults := platformStageRenameOps()
	if defaults.maximum != 8 || defaults.window.Seconds() != 2 {
		t.Fatal("production budget drift")
	}
}
