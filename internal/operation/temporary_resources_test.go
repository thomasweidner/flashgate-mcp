package operation

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCleanupOutcomeValid(t *testing.T) {
	for _, outcome := range []CleanupOutcome{CleanupSucceeded, CleanupFailed, CleanupCancelled, CleanupTimedOut} {
		if !outcome.Valid() {
			t.Fatalf("expected %q to be valid", outcome)
		}
	}
	if CleanupOutcome("unknown").Valid() {
		t.Fatal("unknown outcome must fail closed")
	}
}

func TestTemporaryResourcesRejectInvalidRegistration(t *testing.T) {
	var resources TemporaryResources
	if err := resources.Register(TemporaryResource{}); err == nil {
		t.Fatal("expected empty resource to be rejected")
	}
	if err := resources.Register(TemporaryResource{ID: "one"}); err == nil {
		t.Fatal("expected nil cleanup function to be rejected")
	}
	resource := TemporaryResource{ID: "one", Cleanup: func(context.Context) error { return nil }}
	if err := resources.Register(resource); err != nil {
		t.Fatalf("register resource: %v", err)
	}
	if err := resources.Register(resource); err == nil {
		t.Fatal("expected duplicate ID to be rejected")
	}
}

func TestTemporaryResourcesCleanupRecordsEveryTerminalOutcome(t *testing.T) {
	for _, outcome := range []CleanupOutcome{CleanupSucceeded, CleanupFailed, CleanupCancelled, CleanupTimedOut} {
		t.Run(string(outcome), func(t *testing.T) {
			var resources TemporaryResources
			if err := resources.Register(TemporaryResource{ID: "resource", Cleanup: func(context.Context) error { return nil }}); err != nil {
				t.Fatalf("register resource: %v", err)
			}
			report, err := resources.Cleanup(context.Background(), outcome)
			if err != nil {
				t.Fatalf("cleanup: %v", err)
			}
			if report.Outcome != outcome || len(report.Markers) != 1 || report.Markers[0].Outcome != outcome {
				t.Fatalf("outcome was not recorded: %#v", report)
			}
		})
	}
}

func TestTemporaryResourcesCleanupMarksIncompleteAndContinues(t *testing.T) {
	var resources TemporaryResources
	var order []string
	wantErr := errors.New("remove failed")
	for _, resource := range []TemporaryResource{
		{ID: "first", Cleanup: func(context.Context) error { order = append(order, "first"); return wantErr }},
		{ID: "second", Cleanup: func(context.Context) error { order = append(order, "second"); return nil }},
	} {
		if err := resources.Register(resource); err != nil {
			t.Fatalf("register %q: %v", resource.ID, err)
		}
	}

	report, err := resources.Cleanup(context.Background(), CleanupTimedOut)
	if !errors.Is(err, wantErr) {
		t.Fatalf("cleanup error = %v, want wrapped %v", err, wantErr)
	}
	if !reflect.DeepEqual(order, []string{"first", "second"}) {
		t.Fatalf("cleanup order = %v", order)
	}
	want := CleanupReport{Outcome: CleanupTimedOut, Markers: []CleanupMarker{
		{ID: "first", Outcome: CleanupTimedOut, Incomplete: true},
		{ID: "second", Outcome: CleanupTimedOut, Incomplete: false},
	}}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("report = %#v, want %#v", report, want)
	}
}

func TestTemporaryResourcesCleanupIsConcurrentAndIdempotent(t *testing.T) {
	var resources TemporaryResources
	var calls atomic.Int32
	if err := resources.Register(TemporaryResource{ID: "one", Cleanup: func(context.Context) error {
		calls.Add(1)
		return nil
	}}); err != nil {
		t.Fatalf("register resource: %v", err)
	}

	const callers = 16
	reports := make(chan CleanupReport, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			report, err := resources.Cleanup(context.Background(), CleanupCancelled)
			if err != nil {
				t.Errorf("cleanup: %v", err)
			}
			reports <- report
		}()
	}
	wg.Wait()
	close(reports)
	if got := calls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
	for report := range reports {
		if report.Outcome != CleanupCancelled || len(report.Markers) != 1 || report.Markers[0].Incomplete {
			t.Fatalf("unexpected report: %#v", report)
		}
		report.Markers[0].ID = "mutated"
	}

	if err := resources.Register(TemporaryResource{ID: "late", Cleanup: func(context.Context) error { return nil }}); err == nil {
		t.Fatal("registration after cleanup must be rejected")
	}
	report, err := resources.Cleanup(context.Background(), CleanupFailed)
	if err != nil {
		t.Fatalf("repeat cleanup: %v", err)
	}
	if report.Outcome != CleanupCancelled || report.Markers[0].ID != "one" {
		t.Fatalf("stored report was mutable or replaced: %#v", report)
	}
}

func TestTemporaryResourcesCleanupRejectsInvalidInputBeforeMutation(t *testing.T) {
	var resources TemporaryResources
	var calls atomic.Int32
	if err := resources.Register(TemporaryResource{ID: "one", Cleanup: func(context.Context) error {
		calls.Add(1)
		return nil
	}}); err != nil {
		t.Fatalf("register resource: %v", err)
	}
	if _, err := resources.Cleanup(nil, CleanupSucceeded); err == nil {
		t.Fatal("nil context must be rejected")
	}
	if _, err := resources.Cleanup(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown outcome must be rejected")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid cleanup input invoked a callback")
	}
}
