package command

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLimiterEnforcesProfileAndGlobalBudgets(t *testing.T) {
	limiter, err := NewLimiter(2, map[string]int{"standard": 1, "automation": 2})
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	releaseStandard, err := limiter.Acquire("standard")
	if err != nil {
		t.Fatalf("Acquire(standard) error = %v", err)
	}
	defer releaseStandard()

	_, err = limiter.Acquire("standard")
	assertLimitScope(t, err, LimitScopeProfile)

	releaseAutomation, err := limiter.Acquire("automation")
	if err != nil {
		t.Fatalf("Acquire(automation) error = %v", err)
	}
	_, err = limiter.Acquire("automation")
	assertLimitScope(t, err, LimitScopeGlobal)

	releaseAutomation()
	releaseAutomation()
	if _, err := limiter.Acquire("automation"); err != nil {
		t.Fatalf("Acquire(automation) after release error = %v", err)
	}
}

func TestLimiterFailsClosedForUnknownProfile(t *testing.T) {
	limiter, err := NewLimiter(1, map[string]int{"standard": 1})
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}
	if _, err := limiter.Acquire("unknown"); err == nil {
		t.Fatal("Acquire(unknown) error = nil, want failure")
	}
}

func TestLimiterCopiesConfiguration(t *testing.T) {
	limits := map[string]int{"standard": 1}
	limiter, err := NewLimiter(1, limits)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}
	limits["standard"] = 100

	release, err := limiter.Acquire("standard")
	if err != nil {
		t.Fatalf("Acquire(standard) error = %v", err)
	}
	defer release()
	_, err = limiter.Acquire("standard")
	assertLimitScope(t, err, LimitScopeGlobal)
}

func TestLimiterRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		global   int
		profiles map[string]int
	}{
		{name: "zero global", profiles: map[string]int{"standard": 1}},
		{name: "no profiles", global: 1},
		{name: "empty profile", global: 1, profiles: map[string]int{"": 1}},
		{name: "zero profile", global: 1, profiles: map[string]int{"standard": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewLimiter(tt.global, tt.profiles); err == nil {
				t.Fatal("NewLimiter() error = nil, want failure")
			}
		})
	}
}

func TestLimiterConcurrentAdmissionNeverExceedsBudget(t *testing.T) {
	const limit = 4
	limiter, err := NewLimiter(limit, map[string]int{"standard": limit})
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	var admitted atomic.Int32
	start := make(chan struct{})
	releases := make(chan func(), 32)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			release, err := limiter.Acquire("standard")
			if err == nil {
				admitted.Add(1)
				releases <- release
				return
			}
			if !errors.Is(err, ErrConcurrencyLimit) {
				t.Errorf("Acquire() error = %v", err)
			}
		}()
	}
	close(start)
	workers.Wait()
	close(releases)

	if got := admitted.Load(); got != limit {
		t.Fatalf("admitted = %d, want %d", got, limit)
	}
	for release := range releases {
		release()
	}
}

func assertLimitScope(t *testing.T, err error, want LimitScope) {
	t.Helper()
	if !errors.Is(err, ErrConcurrencyLimit) {
		t.Fatalf("error = %v, want ErrConcurrencyLimit", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("error = %T, want *LimitError", err)
	}
	if limitErr.Scope != want {
		t.Fatalf("scope = %q, want %q", limitErr.Scope, want)
	}
}
