// Package command contains policy primitives for typed command execution.
package command

import (
	"errors"
	"fmt"
	"sync"
)

// ErrConcurrencyLimit is returned when admitting a command would exceed a
// configured global or profile-specific concurrency limit.
var ErrConcurrencyLimit = errors.New("command concurrency limit reached")

// LimitScope identifies the budget that rejected command admission.
type LimitScope string

const (
	LimitScopeGlobal  LimitScope = "global"
	LimitScopeProfile LimitScope = "profile"
)

// LimitError reports which concurrency budget rejected command admission.
// It intentionally contains no command, principal, executable, or host data.
type LimitError struct {
	Scope LimitScope
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%s: %s", ErrConcurrencyLimit, e.Scope)
}

// Unwrap supports errors.Is(err, ErrConcurrencyLimit).
func (e *LimitError) Unwrap() error { return ErrConcurrencyLimit }

// Limiter enforces process-wide and effective-profile command concurrency
// budgets. Admission is deliberately non-blocking so rejected work cannot
// create an unbounded queue inside the command domain.
type Limiter struct {
	mu            sync.Mutex
	globalLimit   int
	profileLimits map[string]int
	globalActive  int
	profileActive map[string]int
}

// NewLimiter constructs a limiter with explicit positive budgets. Every
// profile that may execute a command must have a configured budget.
func NewLimiter(globalLimit int, profileLimits map[string]int) (*Limiter, error) {
	if globalLimit <= 0 {
		return nil, fmt.Errorf("global command concurrency limit must be positive")
	}
	if len(profileLimits) == 0 {
		return nil, fmt.Errorf("at least one profile command concurrency limit is required")
	}

	limits := make(map[string]int, len(profileLimits))
	for profile, limit := range profileLimits {
		if profile == "" {
			return nil, fmt.Errorf("command concurrency profile must not be empty")
		}
		if limit <= 0 {
			return nil, fmt.Errorf("command concurrency limit for profile %q must be positive", profile)
		}
		limits[profile] = limit
	}

	return &Limiter{
		globalLimit:   globalLimit,
		profileLimits: limits,
		profileActive: make(map[string]int, len(limits)),
	}, nil
}

// Acquire admits one command for profile and returns an idempotent release
// function. Unknown profiles fail closed.
func (l *Limiter) Acquire(profile string) (release func(), err error) {
	if l == nil {
		return nil, fmt.Errorf("command concurrency limiter is required")
	}

	l.mu.Lock()
	profileLimit, ok := l.profileLimits[profile]
	if !ok {
		l.mu.Unlock()
		return nil, fmt.Errorf("command concurrency profile is not configured")
	}
	if l.globalActive >= l.globalLimit {
		l.mu.Unlock()
		return nil, &LimitError{Scope: LimitScopeGlobal}
	}
	if l.profileActive[profile] >= profileLimit {
		l.mu.Unlock()
		return nil, &LimitError{Scope: LimitScopeProfile}
	}
	l.globalActive++
	l.profileActive[profile]++
	l.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.globalActive--
			l.profileActive[profile]--
		})
	}, nil
}
