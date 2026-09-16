package managedprocess

import (
	"errors"
	"sync"
)

const defaultProfile = "default"

var (
	ErrInvalidProcessLimits = errors.New("invalid managed process limits")
	ErrProcessLimitReached  = errors.New("managed process limit reached")
)

// Limits bounds simultaneously active managed processes. Global and
// per-profile limits must both be positive so a configuration error cannot
// silently disable either denial-of-service boundary.
type Limits struct {
	Global     int
	PerProfile int
}

// DefaultLimits returns the bounded standalone-engine defaults. Deployments
// may supply stricter values from their effective configuration.
func DefaultLimits() Limits { return Limits{Global: 64, PerProfile: 8} }

type limiter struct {
	mu       sync.Mutex
	limits   Limits
	global   int
	profiles map[string]int
}

func newLimiter(limits Limits) (*limiter, error) {
	if limits.Global <= 0 || limits.PerProfile <= 0 || limits.PerProfile > limits.Global {
		return nil, ErrInvalidProcessLimits
	}
	return &limiter{limits: limits, profiles: make(map[string]int)}, nil
}

func (l *limiter) acquire(profile string) bool {
	if profile == "" {
		profile = defaultProfile
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global >= l.limits.Global || l.profiles[profile] >= l.limits.PerProfile {
		return false
	}
	l.global++
	l.profiles[profile]++
	return true
}

func (l *limiter) release(profile string) {
	if profile == "" {
		profile = defaultProfile
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global <= 0 || l.profiles[profile] <= 0 {
		panic("managed process limit released without reservation")
	}
	l.global--
	l.profiles[profile]--
	if l.profiles[profile] == 0 {
		delete(l.profiles, profile)
	}
}
