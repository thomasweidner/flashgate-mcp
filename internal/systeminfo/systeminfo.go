// Package systeminfo exposes the small, privacy-safe set of host facts that
// FlashGate deliberately makes available to clients.
package systeminfo

import "runtime"

// Info contains the host facts released by the system information domain.
// It intentionally excludes host names, user names, environment variables,
// network data, and other machine identifiers.
type Info struct {
	OS           string
	Architecture string
	Version      string
}

// Provider returns system information.
type Provider interface {
	Info() (Info, error)
}

// RuntimeProvider obtains system information from the current host.
type RuntimeProvider struct{}

// NewProvider creates the production system information provider.
func NewProvider() RuntimeProvider { return RuntimeProvider{} }

// Info returns the explicitly released operating-system facts.
func (RuntimeProvider) Info() (Info, error) {
	version, err := platformVersion()
	if err != nil {
		return Info{}, err
	}
	return Info{OS: runtime.GOOS, Architecture: runtime.GOARCH, Version: version}, nil
}
