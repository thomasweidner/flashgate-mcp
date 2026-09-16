package managedprocess

import (
	"errors"
	"fmt"
	"time"
)

var ErrInvalidRuntimeLimits = errors.New("invalid managed process runtime limits")

// RuntimeLimits defines the default and absolute maximum lifetime of managed
// processes. Both values are mandatory and the default cannot exceed Maximum.
type RuntimeLimits struct {
	Default time.Duration
	Maximum time.Duration
}

// DefaultRuntimeLimits returns bounded standalone-engine runtime limits.
func DefaultRuntimeLimits() RuntimeLimits {
	return RuntimeLimits{Default: 15 * time.Minute, Maximum: 24 * time.Hour}
}

func (limits RuntimeLimits) validate() error {
	if limits.Default <= 0 || limits.Maximum <= 0 || limits.Default > limits.Maximum {
		return ErrInvalidRuntimeLimits
	}
	return nil
}

func (limits RuntimeLimits) effective(requested time.Duration) (time.Duration, error) {
	if requested < 0 || requested > limits.Maximum {
		return 0, fmt.Errorf("%w: runtime must be between zero and %s", ErrInvalidLaunch, limits.Maximum)
	}
	if requested == 0 {
		return limits.Default, nil
	}
	return requested, nil
}
