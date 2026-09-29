package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingOwner struct {
	name  string
	calls *[]string
	err   error
}

func (o recordingOwner) Shutdown(context.Context) error {
	*o.calls = append(*o.calls, o.name)
	return o.err
}

func TestCoordinatorCancelsAndShutsDownOwnersOnceInReverseOrder(t *testing.T) {
	t.Parallel()

	calls := []string{}
	wantErr := errors.New("cleanup failed")
	coordinator := New(
		context.Background(),
		recordingOwner{name: "jobs", calls: &calls, err: wantErr},
		recordingOwner{name: "managed-processes", calls: &calls},
	)

	if err := coordinator.Shutdown(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("expected cleanup error, got %v", err)
	}
	if err := coordinator.Shutdown(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("expected stable cleanup error, got %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"managed-processes", "jobs"}) {
		t.Fatalf("unexpected shutdown calls: %v", calls)
	}
	if err := coordinator.Context().Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected process-root cancellation, got %v", err)
	}
}
