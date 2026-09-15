//go:build !linux && !windows

package process

import "context"

func listProcesses(context.Context) ([]Entry, error) {
	return nil, ErrObservationUnsupported
}

func processDetails(context.Context, uint32) (Details, error) {
	return Details{}, ErrObservationUnsupported
}
