package resource

import (
	"errors"
	"sync"
)

var (
	ErrLimitExceeded  = errors.New("resource limit exceeded")
	ErrNotEnforceable = errors.New("required resource enforcement unavailable")
)

// Limits is an independent value snapshot. Zero values deny consumption.
// A root's stored snapshot is never exposed by pointer or mutable reference.
type Limits struct {
	File, Write, Scan, Result, Temp int64
}

func (l Limits) Validate() error {
	for _, n := range [...]int64{l.File, l.Write, l.Scan, l.Result, l.Temp} {
		if n <= 0 || n == 1<<63-1 {
			return ErrLimitExceeded
		}
	}
	return nil
}

// Within rejects configuration expansion of a global hard ceiling.
func (l Limits) Within(global Limits) bool {
	return l.Validate() == nil && global.Validate() == nil &&
		l.File <= global.File && l.Write <= global.Write && l.Scan <= global.Scan &&
		l.Result <= global.Result && l.Temp <= global.Temp
}

type Dimension uint8

const (
	File Dimension = iota
	Write
	Scan
	Result
	Temp
	DimensionCount
)

func (l Limits) Cap(d Dimension) int64 {
	switch d {
	case File:
		return l.File
	case Write:
		return l.Write
	case Scan:
		return l.Scan
	case Result:
		return l.Result
	case Temp:
		return l.Temp
	default:
		return 0
	}
}

func (l Limits) Check(d Dimension, n int64) error {
	if d >= DimensionCount || n < 0 || n > l.Cap(d) {
		return ErrLimitExceeded
	}
	return nil
}

// Meter belongs to one root and operation, never to a mutable client policy.
// Charges are cumulative; temporary reservations are live and releasable.
type Meter struct {
	mu     sync.Mutex
	limits Limits
	used   [DimensionCount]int64
}

func NewMeter(l Limits) (*Meter, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return &Meter{limits: l}, nil
}

func (m *Meter) Charge(d Dimension, n int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d >= DimensionCount || n < 0 || n > m.limits.Cap(d)-m.used[d] {
		return ErrLimitExceeded
	}
	m.used[d] += n
	return nil
}

// ReserveTemp charges before scratch allocation. The returned release is
// idempotent and safe to defer through cancellation and all failure paths.
func (m *Meter) ReserveTemp(n int64) (func(), error) {
	if err := m.Charge(Temp, n); err != nil {
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.used[Temp] -= n
		})
	}, nil
}

// Enforcement is implemented by a backend's actual admission boundary.
// Implementations must prove hard enforcement for each requested dimension;
// configured values or executable presence alone cannot satisfy this contract.
// Productive process enforcement belongs to the later execution owners.
type Enforcement interface {
	RequireHardLimits(Limits, []Dimension) error
}

// Admit performs no work when the chosen backend cannot enforce the operation's
// declared limits. An implementation must check required dimensions as well as
// effective values; it cannot silently substitute a weaker fallback.
func Admit(backend Enforcement, limits Limits, required ...Dimension) error {
	if backend == nil || limits.Validate() != nil || len(required) == 0 {
		return ErrNotEnforceable
	}
	for _, d := range required {
		if d >= DimensionCount {
			return ErrNotEnforceable
		}
	}
	return backend.RequireHardLimits(limits, required)
}
