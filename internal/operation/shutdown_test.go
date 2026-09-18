package operation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewShutdownRejectsInvalidBounds(t *testing.T) {
	for _, config := range []ShutdownConfig{{}, {GracePeriod: time.Second}, {ForcePeriod: time.Second}} {
		if _, err := NewShutdown(config); !errors.Is(err, ErrInvalidShutdownConfig) {
			t.Fatalf("NewShutdown(%+v) error = %v", config, err)
		}
	}
}

func TestShutdownGracefullyCancelsInRegistrationOrder(t *testing.T) {
	shutdown := newTestShutdown(t)
	var mu sync.Mutex
	var order []string
	for _, id := range []string{"first", "second"} {
		id := id
		done := make(chan struct{})
		if err := shutdown.Register(ShutdownParticipant{ID: id, Done: done, Cancel: func() {
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			close(done)
		}}); err != nil {
			t.Fatal(err)
		}
	}

	report, err := shutdown.Run(context.Background())
	if err != nil || !report.Complete || len(report.Outcomes) != 2 {
		t.Fatalf("Run() = %+v, %v", report, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if order[0] != "first" || order[1] != "second" {
		t.Fatalf("cancellation order = %v", order)
	}
	for _, outcome := range report.Outcomes {
		if !outcome.Terminated || outcome.Forced {
			t.Fatalf("unexpected outcome: %+v", outcome)
		}
	}
}

func TestShutdownEscalatesAndReportsIncompleteWorkers(t *testing.T) {
	shutdown, err := NewShutdown(ShutdownConfig{GracePeriod: 5 * time.Millisecond, ForcePeriod: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	forceErr := errors.New("force failed")
	if err := shutdown.Register(ShutdownParticipant{ID: "worker", Done: done, Cancel: func() {}, Force: func() error { return forceErr }}); err != nil {
		t.Fatal(err)
	}
	report, err := shutdown.Run(context.Background())
	if err != nil || report.Complete || len(report.Outcomes) != 1 {
		t.Fatalf("Run() = %+v, %v", report, err)
	}
	outcome := report.Outcomes[0]
	if outcome.Terminated || !outcome.Forced || !errors.Is(outcome.ForceError, forceErr) {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
}

func TestShutdownIsConcurrentAndReportIsIsolated(t *testing.T) {
	shutdown := newTestShutdown(t)
	done := make(chan struct{})
	var cancels atomic.Int32
	if err := shutdown.Register(ShutdownParticipant{ID: "worker", Done: done, Cancel: func() { cancels.Add(1); close(done) }}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	reports := make([]ShutdownReport, 20)
	for i := range reports {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reports[i], _ = shutdown.Run(context.Background())
		}(i)
	}
	wg.Wait()
	if cancels.Load() != 1 {
		t.Fatalf("Cancel called %d times", cancels.Load())
	}
	reports[0].Outcomes[0].ID = "changed"
	if reports[1].Outcomes[0].ID != "worker" {
		t.Fatal("reports share mutable outcome storage")
	}
	if err := shutdown.Register(ShutdownParticipant{ID: "late", Done: done, Cancel: func() {}}); !errors.Is(err, ErrShutdownStarted) {
		t.Fatalf("late Register() error = %v", err)
	}
}

func TestShutdownCallerContextDoesNotStopShutdown(t *testing.T) {
	shutdown := newTestShutdown(t)
	done := make(chan struct{})
	if err := shutdown.Register(ShutdownParticipant{ID: "worker", Done: done, Cancel: func() {}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := shutdown.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
	close(done)
	report, err := shutdown.Run(context.Background())
	if err != nil || !report.Complete {
		t.Fatalf("second Run() = %+v, %v", report, err)
	}
}

func TestShutdownRejectsInvalidAndDuplicateParticipants(t *testing.T) {
	shutdown := newTestShutdown(t)
	done := make(chan struct{})
	valid := ShutdownParticipant{ID: "worker", Done: done, Cancel: func() {}}
	for _, participant := range []ShutdownParticipant{{}, {ID: "worker", Done: done}, {ID: "worker", Cancel: func() {}}} {
		if err := shutdown.Register(participant); err == nil {
			t.Fatalf("Register(%+v) succeeded", participant)
		}
	}
	if err := shutdown.Register(valid); err != nil {
		t.Fatal(err)
	}
	if err := shutdown.Register(valid); !errors.Is(err, ErrDuplicateParticipant) {
		t.Fatalf("duplicate Register() error = %v", err)
	}
}

func newTestShutdown(t *testing.T) *Shutdown {
	t.Helper()
	shutdown, err := NewShutdown(ShutdownConfig{GracePeriod: time.Second, ForcePeriod: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return shutdown
}
