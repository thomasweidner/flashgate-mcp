//go:build !windows && !linux

package execution

func currentProcess(bool) (Identity, []GroupID, error) { return Identity{}, nil, ErrUnsupported }
