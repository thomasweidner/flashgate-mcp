package config

import "errors"

// ErrorCategory identifies an internal startup configuration failure.
type ErrorCategory string

const (
	CategoryMissingRoot              ErrorCategory = "missing_root"
	CategoryInvalidRoot              ErrorCategory = "invalid_root"
	CategoryRootNotFound             ErrorCategory = "root_not_found"
	CategoryRootNotDirectory         ErrorCategory = "root_not_directory"
	CategoryRootNotAllowed           ErrorCategory = "root_not_allowed"
	CategoryInvalidProfile           ErrorCategory = "invalid_profile"
	CategoryInvalidRiskPolicy        ErrorCategory = "invalid_risk_policy"
	CategoryInvalidDevelopmentOption ErrorCategory = "invalid_development_option"
	CategoryStartupFailed            ErrorCategory = "startup_failed"
)

// ConfigError carries a safe category and an optional internal cause.
// Error deliberately omits the cause so callers cannot accidentally expose
// host paths or raw operating-system details.
type ConfigError struct {
	Category ErrorCategory
	Cause    error
}

// Error returns the safe category only.
func (e *ConfigError) Error() string {
	if e == nil {
		return string(CategoryStartupFailed)
	}

	return string(e.Category)
}

// Unwrap exposes the internal cause for errors.Is/errors.As without including
// it in user-facing text.
func (e *ConfigError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}

// Is matches configuration errors by category.
func (e *ConfigError) Is(target error) bool {
	targetError, ok := target.(*ConfigError)
	return ok && e != nil && e.Category == targetError.Category
}

// NewError creates an internal categorized configuration error.
func NewError(category ErrorCategory, cause error) error {
	return &ConfigError{Category: category, Cause: cause}
}

// CategoryOf returns the category carried by err.
func CategoryOf(err error) (ErrorCategory, bool) {
	var configError *ConfigError
	if !errors.As(err, &configError) {
		return "", false
	}

	return configError.Category, true
}
