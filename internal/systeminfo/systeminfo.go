// Package systeminfo exposes the small, privacy-safe set of host facts that
// FlashGate deliberately makes available to clients.
package systeminfo

import (
	"os"
	"runtime"
)

// releasedEnvironmentVariables is the complete environment-variable allowlist.
// The values describe process locale and terminal behavior without exposing
// paths, identities, credentials, or application configuration.
var releasedEnvironmentVariables = [...]string{"COLORTERM", "LANG", "LC_ALL", "LC_CTYPE", "TERM"}

// ReleasedEnvironmentVariables returns a copy of the fixed public allowlist.
func ReleasedEnvironmentVariables() []string {
	result := make([]string, len(releasedEnvironmentVariables))
	copy(result, releasedEnvironmentVariables[:])
	return result
}

// Info contains the host facts released by the system information domain.
// It intentionally excludes host names, user names, unrestricted environment
// variables, network data, and other machine identifiers.
type Info struct {
	OS           string
	Architecture string
	Version      string
	Environment  map[string]string
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
	return Info{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Version:      version,
		Environment:  filteredEnvironment(os.LookupEnv),
	}, nil
}

func filteredEnvironment(lookup func(string) (string, bool)) map[string]string {
	result := make(map[string]string)
	for _, name := range releasedEnvironmentVariables {
		if value, ok := lookup(name); ok && value != "" {
			result[name] = value
		}
	}
	return result
}
