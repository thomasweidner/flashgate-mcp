package managedprocess

import (
	"errors"
	"sync"
	"testing"
)

func TestStatusContract(t *testing.T) {
	tests := []struct {
		status   Status
		terminal bool
	}{
		{StatusStarting, false},
		{StatusRunning, false},
		{StatusExited, true},
		{StatusFailed, true},
		{StatusStopped, true},
		{StatusTimedOut, true},
	}

	for _, test := range tests {
		if !test.status.Valid() {
			t.Errorf("status %q is not valid", test.status)
		}
		if got := test.status.Terminal(); got != test.terminal {
			t.Errorf("status %q Terminal() = %t, want %t", test.status, got, test.terminal)
		}
	}

	for _, status := range []Status{"", "queued", "completed", "timed-out"} {
		if status.Valid() {
			t.Errorf("unexpected valid status %q", status)
		}
		if status.Terminal() {
			t.Errorf("unexpected terminal status %q", status)
		}
	}
}

func TestStateTransitions(t *testing.T) {
	terminalStatuses := []Status{StatusExited, StatusFailed, StatusStopped, StatusTimedOut}

	for _, terminal := range terminalStatuses {
		t.Run("starting_to_"+string(terminal), func(t *testing.T) {
			state := NewState()
			if got := state.Status(); got != StatusStarting {
				t.Fatalf("initial status = %q, want %q", got, StatusStarting)
			}
			if err := state.Transition(terminal); err != nil {
				t.Fatalf("Transition(%q) error = %v", terminal, err)
			}
			if got := state.Status(); got != terminal {
				t.Fatalf("status = %q, want %q", got, terminal)
			}
		})

		t.Run("running_to_"+string(terminal), func(t *testing.T) {
			state := NewState()
			if err := state.Transition(StatusRunning); err != nil {
				t.Fatalf("Transition(%q) error = %v", StatusRunning, err)
			}
			if err := state.Transition(terminal); err != nil {
				t.Fatalf("Transition(%q) error = %v", terminal, err)
			}
			if got := state.Status(); got != terminal {
				t.Fatalf("status = %q, want %q", got, terminal)
			}
		})
	}
}

func TestStateRejectsInvalidAndBackwardTransitions(t *testing.T) {
	tests := []struct {
		name    string
		initial Status
		next    Status
		wantErr error
	}{
		{"unknown", StatusStarting, "unknown", ErrInvalidStatus},
		{"starting_again", StatusStarting, StatusStarting, ErrInvalidStatusTransition},
		{"running_again", StatusRunning, StatusRunning, ErrInvalidStatusTransition},
		{"running_to_starting", StatusRunning, StatusStarting, ErrInvalidStatusTransition},
		{"terminal_to_running", StatusExited, StatusRunning, ErrInvalidStatusTransition},
		{"replace_terminal", StatusFailed, StatusStopped, ErrInvalidStatusTransition},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := NewState()
			if test.initial != StatusStarting {
				if test.initial != StatusRunning {
					if err := state.Transition(test.initial); err != nil {
						t.Fatalf("set initial status %q: %v", test.initial, err)
					}
				} else if err := state.Transition(StatusRunning); err != nil {
					t.Fatalf("set running status: %v", err)
				}
			}

			before := state.Status()
			err := state.Transition(test.next)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Transition(%q) error = %v, want %v", test.next, err, test.wantErr)
			}
			if got := state.Status(); got != before {
				t.Fatalf("failed transition changed status from %q to %q", before, got)
			}
		})
	}
}

func TestStateZeroValueIsUsable(t *testing.T) {
	var state State
	if got := state.Status(); got != StatusStarting {
		t.Fatalf("zero-value status = %q, want %q", got, StatusStarting)
	}
	if err := state.Transition(StatusRunning); err != nil {
		t.Fatalf("zero-value Transition(%q) error = %v", StatusRunning, err)
	}
}

func TestStateOnlyAcceptsOneCompetingTerminalOutcome(t *testing.T) {
	state := NewState()
	if err := state.Transition(StatusRunning); err != nil {
		t.Fatalf("Transition(%q) error = %v", StatusRunning, err)
	}

	terminalStatuses := []Status{StatusExited, StatusFailed, StatusStopped, StatusTimedOut}
	start := make(chan struct{})
	results := make(chan error, len(terminalStatuses))
	var group sync.WaitGroup
	for _, status := range terminalStatuses {
		group.Add(1)
		go func(status Status) {
			defer group.Done()
			<-start
			results <- state.Transition(status)
		}(status)
	}
	close(start)
	group.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Errorf("competing terminal transition error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful terminal transitions = %d, want 1", successes)
	}
	if got := state.Status(); !got.Terminal() {
		t.Fatalf("final status = %q, want terminal status", got)
	}
}
