package command

import (
	"encoding/base64"
	"errors"
	"fmt"
)

const maxResultDiagnostics = 8

// ResultStatus is the closed terminal state of one command invocation.
type ResultStatus string

const (
	ResultSucceeded     ResultStatus = "succeeded"
	ResultFailed        ResultStatus = "failed"
	ResultTimedOut      ResultStatus = "timed_out"
	ResultCanceled      ResultStatus = "canceled"
	ResultStartFailed   ResultStatus = "start_failed"
	ResultCleanupFailed ResultStatus = "cleanup_failed"
)

// DiagnosticCode is a bounded, non-sensitive condition attached to a result.
// It deliberately carries no arbitrary operating-system or process text.
type DiagnosticCode string

const (
	DiagnosticStdoutTruncated       DiagnosticCode = "stdout_truncated"
	DiagnosticStderrTruncated       DiagnosticCode = "stderr_truncated"
	DiagnosticTerminationIncomplete DiagnosticCode = "termination_incomplete"
	DiagnosticCleanupIncomplete     DiagnosticCode = "cleanup_incomplete"
)

// OutputReference is the stable, safely serializable reference to one bounded
// output snapshot. Data is base64 so arbitrary process bytes remain valid JSON.
type OutputReference struct {
	Encoding      string `json:"encoding"`
	Data          string `json:"data"`
	RetainedBytes int64  `json:"retainedBytes"`
	TotalBytes    int64  `json:"totalBytes"`
	Truncated     bool   `json:"truncated"`
}

// Result is the stable public command-result domain object. ExitCode is
// present only when the process produced an observed exit code. TimedOut is
// explicit rather than inferred by clients from an exit code.
type Result struct {
	CommandID   string           `json:"commandId"`
	Status      ResultStatus     `json:"status"`
	ExitCode    *int             `json:"exitCode,omitempty"`
	TimedOut    bool             `json:"timedOut"`
	Stdout      OutputReference  `json:"stdout"`
	Stderr      OutputReference  `json:"stderr"`
	Diagnostics []DiagnosticCode `json:"diagnostics"`
}

// ErrInvalidResult indicates an internally inconsistent command result. Such
// a result must be rejected before it reaches an MCP adapter.
var ErrInvalidResult = errors.New("invalid command result")

// NewResult validates terminal-state invariants and copies all supplied data.
func NewResult(commandID string, status ResultStatus, exitCode *int, stdout, stderr OutputSnapshot, diagnostics []DiagnosticCode) (Result, error) {
	if !validIdentifier(commandID) {
		return Result{}, fmt.Errorf("%w: invalid command id", ErrInvalidResult)
	}
	if err := validateTerminalState(status, exitCode); err != nil {
		return Result{}, err
	}
	if err := validateSnapshot(stdout); err != nil {
		return Result{}, fmt.Errorf("%w: stdout: %v", ErrInvalidResult, err)
	}
	if err := validateSnapshot(stderr); err != nil {
		return Result{}, fmt.Errorf("%w: stderr: %v", ErrInvalidResult, err)
	}
	if err := validateDiagnostics(diagnostics); err != nil {
		return Result{}, err
	}

	result := Result{
		CommandID:   commandID,
		Status:      status,
		TimedOut:    status == ResultTimedOut,
		Stdout:      outputReference(stdout),
		Stderr:      outputReference(stderr),
		Diagnostics: append([]DiagnosticCode(nil), diagnostics...),
	}
	if exitCode != nil {
		code := *exitCode
		result.ExitCode = &code
	}
	if result.Diagnostics == nil {
		result.Diagnostics = []DiagnosticCode{}
	}
	return result, nil
}

func validateTerminalState(status ResultStatus, exitCode *int) error {
	switch status {
	case ResultSucceeded:
		if exitCode == nil || *exitCode != 0 {
			return fmt.Errorf("%w: succeeded requires exit code zero", ErrInvalidResult)
		}
	case ResultFailed:
		if exitCode == nil || *exitCode == 0 {
			return fmt.Errorf("%w: failed requires a nonzero exit code", ErrInvalidResult)
		}
	case ResultTimedOut, ResultCanceled, ResultStartFailed:
		if exitCode != nil {
			return fmt.Errorf("%w: %s cannot include an exit code", ErrInvalidResult, status)
		}
	case ResultCleanupFailed:
		// Cleanup can fail after the process exited or while its exit is unknown.
	default:
		return fmt.Errorf("%w: unknown status", ErrInvalidResult)
	}
	return nil
}

func validateSnapshot(snapshot OutputSnapshot) error {
	if snapshot.TotalBytes < 0 || int64(len(snapshot.Bytes)) > snapshot.TotalBytes {
		return errors.New("invalid byte counts")
	}
	if snapshot.Truncated != (int64(len(snapshot.Bytes)) < snapshot.TotalBytes) {
		return errors.New("truncation marker does not match byte counts")
	}
	return nil
}

func validateDiagnostics(diagnostics []DiagnosticCode) error {
	if len(diagnostics) > maxResultDiagnostics {
		return fmt.Errorf("%w: too many diagnostics", ErrInvalidResult)
	}
	seen := make(map[DiagnosticCode]struct{}, len(diagnostics))
	for _, diagnostic := range diagnostics {
		switch diagnostic {
		case DiagnosticStdoutTruncated, DiagnosticStderrTruncated, DiagnosticTerminationIncomplete, DiagnosticCleanupIncomplete:
		default:
			return fmt.Errorf("%w: unknown diagnostic", ErrInvalidResult)
		}
		if _, exists := seen[diagnostic]; exists {
			return fmt.Errorf("%w: duplicate diagnostic", ErrInvalidResult)
		}
		seen[diagnostic] = struct{}{}
	}
	return nil
}

func outputReference(snapshot OutputSnapshot) OutputReference {
	return OutputReference{
		Encoding:      "base64",
		Data:          base64.StdEncoding.EncodeToString(snapshot.Bytes),
		RetainedBytes: int64(len(snapshot.Bytes)),
		TotalBytes:    snapshot.TotalBytes,
		Truncated:     snapshot.Truncated,
	}
}
