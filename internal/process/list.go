// Package process provides bounded, platform-neutral process observation.
package process

import (
	"context"
	"errors"
	"sort"
)

// ErrObservationUnsupported indicates that the current platform has no process
// observation adapter.
var ErrObservationUnsupported = errors.New("process observation unsupported")

// Entry is the deliberately small, portable process-list representation.
// Additional fields belong to BL-115 and BL-116.
type Entry struct {
	PID  uint32 `json:"pid"`
	Name string `json:"name"`
}

// Lister obtains a point-in-time process snapshot in strictly increasing PID
// order.
type Lister interface {
	List(context.Context) ([]Entry, error)
}

// LocalLister observes processes through the native operating-system adapter.
type LocalLister struct{}

// List returns a PID-ordered point-in-time snapshot.
func (LocalLister) List(ctx context.Context) ([]Entry, error) {
	entries, err := listProcesses(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].PID == entries[j].PID {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].PID < entries[j].PID
	})
	return entries, nil
}
