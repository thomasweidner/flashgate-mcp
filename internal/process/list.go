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

// ErrNotFound indicates that the requested PID was not present when observed.
var ErrNotFound = errors.New("process not found")

// ErrAccessDenied indicates that the operating system refused process metadata
// access. Callers must not expose the underlying platform error.
var ErrAccessDenied = errors.New("process access denied")

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

// Details is the bounded, portable process-detail representation. It
// deliberately excludes command lines, environments, users, executable paths,
// and working directories.
type Details struct {
	PID         uint32
	Name        string
	ParentPID   uint32
	ThreadCount uint32
}

// Detailer obtains portable details for one PID.
type Detailer interface {
	Details(context.Context, uint32) (Details, error)
}

// TreeObserver obtains portable parent relationships for a point-in-time
// process snapshot. Entries may be absent when native metadata is unavailable.
type TreeSnapshot struct {
	Processes []Details
	Partial   bool
}

type TreeObserver interface {
	Tree(context.Context) (TreeSnapshot, error)
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

// LocalDetailer observes one process through the native operating-system
// adapter.
type LocalDetailer struct{}

// Details returns a point-in-time portable detail snapshot for pid.
func (LocalDetailer) Details(ctx context.Context, pid uint32) (Details, error) {
	return processDetails(ctx, pid)
}

// LocalTreeObserver observes native process parent relationships.
type LocalTreeObserver struct{}

// Tree returns a point-in-time process snapshot in deterministic PID order.
func (LocalTreeObserver) Tree(ctx context.Context) (TreeSnapshot, error) {
	snapshot, err := processTree(ctx)
	if err != nil {
		return TreeSnapshot{}, err
	}
	sort.Slice(snapshot.Processes, func(i, j int) bool { return snapshot.Processes[i].PID < snapshot.Processes[j].PID })
	return snapshot, nil
}
