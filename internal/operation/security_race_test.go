package operation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLeakRegistryCleanupCannotDeleteReplacementJob(t *testing.T) {
	registry := NewLeakRegistry()
	now := time.Now()
	started := make(chan struct{})
	release := make(chan struct{})
	old := LeakRecord{
		ID:        "opaque-handle",
		Owner:     "first-principal",
		ExpiresAt: now,
		Cleanup: func(context.Context) error {
			close(started)
			<-release
			return nil
		},
	}
	if err := registry.Register(old); err != nil {
		t.Fatal(err)
	}

	sweepDone := make(chan []LeakOutcome)
	go func() { sweepDone <- registry.Sweep(context.Background(), now) }()
	<-started
	if err := registry.Remove(old.ID, old.Owner); err != nil {
		t.Fatal(err)
	}
	replacement := LeakRecord{
		ID:        old.ID,
		Owner:     "replacement-principal",
		ExpiresAt: now.Add(time.Hour),
		Cleanup:   func(context.Context) error { return nil },
	}
	if err := registry.Register(replacement); err != nil {
		t.Fatal(err)
	}
	close(release)
	if outcomes := <-sweepDone; len(outcomes) != 1 || !outcomes[0].Cleaned {
		t.Fatalf("Sweep() = %+v", outcomes)
	}

	if err := registry.CheckOwner(replacement.ID, replacement.Owner); err != nil {
		t.Fatalf("replacement job was removed by stale cleanup: %v", err)
	}
	if metrics := registry.Metrics(); metrics.Active != 1 || metrics.Cleaned != 0 {
		t.Fatalf("Metrics() = %+v", metrics)
	}
}

func TestLeakRegistryConcurrentOwnerChecksNeverAuthorizeAttackers(t *testing.T) {
	registry := NewLeakRegistry()
	const attempts = 64
	if err := registry.Register(LeakRecord{
		ID:        "unguessable-handle",
		Owner:     "authorized-principal",
		ExpiresAt: time.Now().Add(time.Hour),
		Cleanup:   func(context.Context) error { return nil },
	}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var unauthorized atomic.Int32
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := registry.CheckOwner("unguessable-handle", "attacker"); !errors.Is(err, ErrJobOwnerMismatch) {
				unauthorized.Add(1)
			}
		}()
	}
	wg.Wait()
	if unauthorized.Load() != 0 {
		t.Fatalf("%d attacker checks did not receive owner mismatch", unauthorized.Load())
	}
	if metrics := registry.Metrics(); metrics.OwnershipDenials != attempts || metrics.Active != 1 {
		t.Fatalf("Metrics() = %+v", metrics)
	}
}

func TestLeakRegistryErrorsDoNotDiscloseIdentifiers(t *testing.T) {
	registry := NewLeakRegistry()
	secretHandle := "op_secret_handle"
	secretOwner := "principal-secret"
	if err := registry.Register(LeakRecord{
		ID:        secretHandle,
		Owner:     secretOwner,
		ExpiresAt: time.Now().Add(time.Hour),
		Cleanup:   func(context.Context) error { return nil },
	}); err != nil {
		t.Fatal(err)
	}

	for _, err := range []error{
		registry.CheckOwner(secretHandle, "attacker-secret"),
		registry.CheckOwner("unknown-secret", secretOwner),
		registry.Remove(secretHandle, "attacker-secret"),
	} {
		if err == nil {
			t.Fatal("security check unexpectedly succeeded")
		}
		for _, secret := range []string{secretHandle, secretOwner, "attacker-secret", "unknown-secret"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error %q disclosed identifier %q", err, secret)
			}
		}
	}
}

func TestShutdownRegisterRaceLeavesNoAcceptedParticipantRunning(t *testing.T) {
	shutdown, err := NewShutdown(ShutdownConfig{GracePeriod: time.Second, ForcePeriod: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	const participants = 64
	start := make(chan struct{})
	var wg sync.WaitGroup
	var accepted atomic.Int32
	var cancelled atomic.Int32
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			done := make(chan struct{})
			err := shutdown.Register(ShutdownParticipant{
				ID:   string(rune('A' + i)),
				Done: done,
				Cancel: func() {
					cancelled.Add(1)
					close(done)
				},
			})
			if err == nil {
				accepted.Add(1)
				return
			}
			if !errors.Is(err, ErrShutdownStarted) {
				t.Errorf("Register() error = %v", err)
			}
		}(i)
	}
	close(start)
	report, err := shutdown.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if !report.Complete || int32(len(report.Outcomes)) != accepted.Load() || cancelled.Load() != accepted.Load() {
		t.Fatalf("report=%+v accepted=%d cancelled=%d", report, accepted.Load(), cancelled.Load())
	}
}
