// Package operation provides transport-neutral lifecycle primitives for
// bounded long-running work.
package operation

import (
	"errors"
	"strings"
	"sync"
)

var (
	ErrInvalidConcurrencyLimits  = errors.New("invalid operation concurrency limits")
	ErrInvalidConcurrencyKey     = errors.New("invalid operation concurrency key")
	ErrGlobalConcurrencyLimit    = errors.New("global operation concurrency limit reached")
	ErrDomainConcurrencyLimit    = errors.New("domain operation concurrency limit reached")
	ErrPrincipalConcurrencyLimit = errors.New("principal operation concurrency limit reached")
)

// ConcurrencyLimits bounds active operations at each shared-service scope.
type ConcurrencyLimits struct {
	Global       int
	PerDomain    int
	PerPrincipal int
}

// DefaultConcurrencyLimits returns conservative, configurable service limits.
func DefaultConcurrencyLimits() ConcurrencyLimits {
	return ConcurrencyLimits{Global: 16, PerDomain: 8, PerPrincipal: 4}
}

// ConcurrencyLimiter atomically reserves capacity at global, domain, and
// principal scopes. It deliberately does not queue work; fair queuing is owned
// by the scheduler rather than this admission primitive.
type ConcurrencyLimiter struct {
	mu          sync.Mutex
	limits      ConcurrencyLimits
	global      int
	byDomain    map[string]int
	byPrincipal map[string]int
}

// NewConcurrencyLimiter creates a limiter with positive limits whose narrower
// scopes cannot exceed the global scope.
func NewConcurrencyLimiter(limits ConcurrencyLimits) (*ConcurrencyLimiter, error) {
	if limits.Global <= 0 || limits.PerDomain <= 0 || limits.PerPrincipal <= 0 ||
		limits.PerDomain > limits.Global || limits.PerPrincipal > limits.Global {
		return nil, ErrInvalidConcurrencyLimits
	}
	return &ConcurrencyLimiter{
		limits:      limits,
		byDomain:    make(map[string]int),
		byPrincipal: make(map[string]int),
	}, nil
}

// TryAcquire reserves one execution slot or returns the first exhausted scope.
func (l *ConcurrencyLimiter) TryAcquire(domain, principal string) (*Permit, error) {
	if l == nil || domain == "" || principal == "" ||
		domain != strings.TrimSpace(domain) || principal != strings.TrimSpace(principal) {
		return nil, ErrInvalidConcurrencyKey
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global >= l.limits.Global {
		return nil, ErrGlobalConcurrencyLimit
	}
	if l.byDomain[domain] >= l.limits.PerDomain {
		return nil, ErrDomainConcurrencyLimit
	}
	if l.byPrincipal[principal] >= l.limits.PerPrincipal {
		return nil, ErrPrincipalConcurrencyLimit
	}

	l.global++
	l.byDomain[domain]++
	l.byPrincipal[principal]++
	return &Permit{limiter: l, domain: domain, principal: principal}, nil
}

// ConcurrencySnapshot is a point-in-time copy of active reservations.
type ConcurrencySnapshot struct {
	Global      int
	ByDomain    map[string]int
	ByPrincipal map[string]int
}

// Snapshot returns counters without exposing mutable limiter state.
func (l *ConcurrencyLimiter) Snapshot() ConcurrencySnapshot {
	if l == nil {
		return ConcurrencySnapshot{ByDomain: map[string]int{}, ByPrincipal: map[string]int{}}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return ConcurrencySnapshot{
		Global:      l.global,
		ByDomain:    cloneCounts(l.byDomain),
		ByPrincipal: cloneCounts(l.byPrincipal),
	}
}

// Permit owns one reservation and releases it at most once.
type Permit struct {
	once      sync.Once
	limiter   *ConcurrencyLimiter
	domain    string
	principal string
}

// Release returns all capacity owned by the permit. It is safe to call concurrently.
func (p *Permit) Release() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		p.limiter.release(p.domain, p.principal)
	})
}

func (l *ConcurrencyLimiter) release(domain, principal string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.global--
	decrementCount(l.byDomain, domain)
	decrementCount(l.byPrincipal, principal)
}

func decrementCount(counts map[string]int, key string) {
	if counts[key] == 1 {
		delete(counts, key)
		return
	}
	counts[key]--
}

func cloneCounts(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
