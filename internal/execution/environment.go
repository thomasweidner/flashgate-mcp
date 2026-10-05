// Package execution contains protocol-independent command execution policy.
package execution

import (
	"errors"
	"sort"
	"strings"
)

var (
	// ErrEnvironmentName is returned for malformed environment variable names.
	ErrEnvironmentName = errors.New("invalid environment variable name")
	// ErrEnvironmentVariableDenied is returned for variables that may alter
	// executable loading, interpreter behavior, hooks, or configuration.
	ErrEnvironmentVariableDenied = errors.New("environment variable is denied")
	// ErrEnvironmentVariableNotAllowed is returned for variables outside the
	// command definition's explicit allowlist.
	ErrEnvironmentVariableNotAllowed = errors.New("environment variable is not allowed")
	// ErrEnvironmentValue is returned for values that cannot be passed safely.
	ErrEnvironmentValue = errors.New("invalid environment variable value")
)

// EnvironmentPolicy builds a complete, explicit child-process environment.
// It never reads or merges the server process environment.
type EnvironmentPolicy struct {
	allowed map[string]string
}

// NewEnvironmentPolicy validates a command definition's environment
// allowlist. Names are compared case-insensitively so a definition has the
// same security outcome on Windows and Unix.
func NewEnvironmentPolicy(allowedNames []string) (EnvironmentPolicy, error) {
	policy := EnvironmentPolicy{allowed: make(map[string]string, len(allowedNames))}
	for _, name := range allowedNames {
		if !validEnvironmentName(name) {
			return EnvironmentPolicy{}, ErrEnvironmentName
		}
		canonical := strings.ToUpper(name)
		if deniedEnvironmentName(canonical) {
			return EnvironmentPolicy{}, ErrEnvironmentVariableDenied
		}
		if _, exists := policy.allowed[canonical]; exists {
			return EnvironmentPolicy{}, ErrEnvironmentName
		}
		policy.allowed[canonical] = name
	}
	return policy, nil
}

// Build validates explicit values and returns deterministic NAME=value entries
// suitable for os/exec.Cmd.Env. An empty result intentionally means an empty
// child environment, not inherited server state.
func (p EnvironmentPolicy) Build(explicit map[string]string) ([]string, error) {
	values := make(map[string]string, len(explicit))
	for name, value := range explicit {
		if !validEnvironmentName(name) {
			return nil, ErrEnvironmentName
		}
		canonical := strings.ToUpper(name)
		configuredName, allowed := p.allowed[canonical]
		if !allowed {
			if deniedEnvironmentName(canonical) {
				return nil, ErrEnvironmentVariableDenied
			}
			return nil, ErrEnvironmentVariableNotAllowed
		}
		if strings.IndexByte(value, 0) >= 0 {
			return nil, ErrEnvironmentValue
		}
		if _, exists := values[canonical]; exists {
			return nil, ErrEnvironmentName
		}
		values[canonical] = configuredName + "=" + value
	}

	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, name := range keys {
		result = append(result, values[name])
	}
	return result, nil
}

func validEnvironmentName(name string) bool {
	if name == "" || strings.IndexByte(name, '=') >= 0 || strings.IndexByte(name, 0) >= 0 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}

func deniedEnvironmentName(name string) bool {
	for _, prefix := range []string{"DYLD_", "GIT_", "LD_", "PYTHON", "RUBY", "PERL", "NODE_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	switch name {
	case "BASH_ENV", "ENV", "IFS", "PROMPT_COMMAND", "SHELLOPTS", "ZDOTDIR":
		return true
	default:
		return false
	}
}
