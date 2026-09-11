package operation

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrInvalidShutdownConfig reports a non-positive shutdown phase timeout.
	ErrInvalidShutdownConfig = errors.New("operation: invalid shutdown configuration")
	// ErrShutdownStarted reports registration after shutdown has begun.
	ErrShutdownStarted = errors.New("operation: shutdown already started")
	// ErrDuplicateParticipant reports reuse of a participant identifier.
	ErrDuplicateParticipant = errors.New("operation: duplicate shutdown participant")
)

// ShutdownParticipant is lifecycle wiring supplied by an owning domain.
// Cancel and Force must return promptly. Done must be closed when the worker
// and its owned resources have terminated. Force may be nil when escalation is
// not available for the execution unit.
type ShutdownParticipant struct {
	ID     string
	Cancel func()
	Done   <-chan struct{}
	Force  func() error
}

// ShutdownConfig bounds both shutdown phases.
type ShutdownConfig struct {
	GracePeriod time.Duration
	ForcePeriod time.Duration
}

// ShutdownOutcome is the final state of one registered participant.
type ShutdownOutcome struct {
	ID         string
	Terminated bool
	Forced     bool
	ForceError error
}

// ShutdownReport is the immutable result of the first shutdown call.
type ShutdownReport struct {
	Outcomes []ShutdownOutcome
	Complete bool
}

// Shutdown coordinates bounded cancellation and escalation without taking
// ownership of domain work.
type Shutdown struct {
	mu           sync.Mutex
	config       ShutdownConfig
	participants []ShutdownParticipant
	ids          map[string]struct{}
	started      bool
	done         chan struct{}
	report       ShutdownReport
}

// NewShutdown constructs a shutdown coordinator with explicit phase bounds.
func NewShutdown(config ShutdownConfig) (*Shutdown, error) {
	if config.GracePeriod <= 0 || config.ForcePeriod <= 0 {
		return nil, ErrInvalidShutdownConfig
	}
	return &Shutdown{config: config, ids: make(map[string]struct{}), done: make(chan struct{})}, nil
}

// Register adds a participant before shutdown starts.
func (s *Shutdown) Register(participant ShutdownParticipant) error {
	if participant.ID == "" || participant.Cancel == nil || participant.Done == nil {
		return errors.New("operation: invalid shutdown participant")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrShutdownStarted
	}
	if _, exists := s.ids[participant.ID]; exists {
		return ErrDuplicateParticipant
	}
	s.ids[participant.ID] = struct{}{}
	s.participants = append(s.participants, participant)
	return nil
}

// Run starts shutdown once. Concurrent and later callers receive the same
// final report. The caller context only bounds that caller's wait; it does not
// weaken the server-owned shutdown bounds.
func (s *Shutdown) Run(ctx context.Context) (ShutdownReport, error) {
	s.mu.Lock()
	if !s.started {
		s.started = true
		participants := append([]ShutdownParticipant(nil), s.participants...)
		go s.execute(participants)
	}
	done := s.done
	s.mu.Unlock()

	select {
	case <-done:
		s.mu.Lock()
		report := cloneShutdownReport(s.report)
		s.mu.Unlock()
		return report, nil
	case <-ctx.Done():
		return ShutdownReport{}, ctx.Err()
	}
}

func (s *Shutdown) execute(participants []ShutdownParticipant) {
	for _, participant := range participants {
		participant.Cancel()
	}

	outcomes := make([]ShutdownOutcome, len(participants))
	for i := range participants {
		outcomes[i].ID = participants[i].ID
	}
	waitForParticipants(participants, outcomes, s.config.GracePeriod)

	for i, participant := range participants {
		if outcomes[i].Terminated || participant.Force == nil {
			continue
		}
		outcomes[i].Forced = true
		outcomes[i].ForceError = participant.Force()
	}
	waitForParticipants(participants, outcomes, s.config.ForcePeriod)

	report := ShutdownReport{Outcomes: outcomes, Complete: true}
	for i := range outcomes {
		if !outcomes[i].Terminated {
			report.Complete = false
		}
	}
	s.mu.Lock()
	s.report = report
	close(s.done)
	s.mu.Unlock()
}

func waitForParticipants(participants []ShutdownParticipant, outcomes []ShutdownOutcome, timeout time.Duration) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		pending := false
		for i, participant := range participants {
			if outcomes[i].Terminated {
				continue
			}
			select {
			case <-participant.Done:
				outcomes[i].Terminated = true
			default:
				pending = true
			}
		}
		if !pending {
			return
		}
		select {
		case <-deadline.C:
			return
		case <-time.After(time.Millisecond):
		}
	}
}

func cloneShutdownReport(report ShutdownReport) ShutdownReport {
	clone := report
	clone.Outcomes = append([]ShutdownOutcome(nil), report.Outcomes...)
	return clone
}
