package operation

import "testing"

func TestStatusClassification(t *testing.T) {
	tests := []struct {
		status   Status
		valid    bool
		terminal bool
	}{
		{StatusQueued, true, false},
		{StatusRunning, true, false},
		{StatusCompleted, true, true},
		{StatusFailed, true, true},
		{StatusCancelled, true, true},
		{StatusTimedOut, true, true},
		{"", false, false},
		{"unknown", false, false},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			if got := test.status.Valid(); got != test.valid {
				t.Fatalf("Valid() = %v, want %v", got, test.valid)
			}
			if got := test.status.Terminal(); got != test.terminal {
				t.Fatalf("Terminal() = %v, want %v", got, test.terminal)
			}
		})
	}
}

func TestStatusTransitions(t *testing.T) {
	statuses := []Status{
		StatusQueued,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
		StatusCancelled,
		StatusTimedOut,
	}
	allowed := map[Status]map[Status]bool{
		StatusQueued: {
			StatusRunning: true, StatusFailed: true, StatusCancelled: true, StatusTimedOut: true,
		},
		StatusRunning: {
			StatusCompleted: true, StatusFailed: true, StatusCancelled: true, StatusTimedOut: true,
		},
	}

	for _, from := range statuses {
		for _, to := range statuses {
			want := allowed[from][to]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%q.CanTransitionTo(%q) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestStatusTransitionsRejectUnknownStates(t *testing.T) {
	if Status("unknown").CanTransitionTo(StatusRunning) {
		t.Fatal("unknown source state must not transition")
	}
	if StatusQueued.CanTransitionTo(Status("unknown")) {
		t.Fatal("known source state must not transition to an unknown state")
	}
}
