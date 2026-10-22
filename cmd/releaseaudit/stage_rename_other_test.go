//go:build !windows

package main

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

func TestStageRenameOtherSingleAttempt(t *testing.T) {
	for _, code := range []syscall.Errno{5, 32} {
		b, verify := stageTestBinding(t)
		ops := platformStageRenameOps()
		calls := 0
		ops.rename = func(string, string) error { calls++; return code }
		ops.sleep = func(time.Duration) { t.Fatal("non-Windows backoff") }
		if err := b.renameWithOps(verify, ops); !errors.Is(err, code) || calls != 1 || ops.maximum != 1 {
			t.Fatalf("calls=%d: %v", calls, err)
		}
	}
}
