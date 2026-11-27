// Package audit records bounded security evidence for trusted core operations.
package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/diagnostics"
)

type Priority uint8

const (
	Critical Priority = iota
	High
	Normal
	Low
	priorityCount
)

type Phase string

const (
	Decision Phase = "decision"
	Intent   Phase = "intent"
	Outcome  Phase = "outcome"
	Progress Phase = "progress"
	Loss     Phase = "loss"
)

type Reason string

const (
	Allowed      Reason = "allowed"
	Denied       Reason = "denied"
	RiskDenied   Reason = "risk_denied"
	GrantDenied  Reason = "grant_denied"
	RootDenied   Reason = "root_denied"
	NativeDenied Reason = "native_denied"
	Invalid      Reason = "invalid"
	Unsupported  Reason = "unsupported"
	Mismatch     Reason = "binding_mismatch"
	Completed    Reason = "completed"
	Failed       Reason = "failed"
	Cancelled    Reason = "cancelled"
	TimedOut     Reason = "timed_out"
	Unavailable  Reason = "audit_unavailable"
	Uncertain    Reason = "uncertain"
)

type EventID string

// Reference is meaningful across server instances. Runtime is the trusted
// BL-236 generation, and Request is the existing request sequence in it.
type Reference struct {
	Runtime string `json:"runtime"`
	Request uint64 `json:"request"`
}

type Operand struct {
	Kind     string `json:"kind"`
	Role     string `json:"role"`
	ID       string `json:"id"`
	Relative string `json:"relative,omitempty"`
}

type Counters struct {
	RequestedBytes uint64  `json:"requested_bytes,omitempty"`
	ReadBytes      *uint64 `json:"read_bytes,omitempty"`
	WrittenBytes   *uint64 `json:"written_bytes,omitempty"`
	Entries        uint64  `json:"entries,omitempty"`
}

// Observation is supplied by a trusted core integration. Set Operands only
// after authorization and include a path only after its field protection gate.
// The recorder projects native principal IDs before encoding the event.
type Observation struct {
	Request      Reference
	Original     *Reference
	Parent       EventID
	Phase        Phase
	Reason       Reason
	Caller       string
	Effective    string
	Backend      string
	Origin       string
	Policy       string
	Profile      string
	Operation    string
	Variant      string
	Risk         uint8
	Requirements []string
	Operands     []Operand
	Elapsed      time.Duration
	Counters     Counters
}

type event struct {
	Version      int         `json:"v"`
	ID           EventID     `json:"id"`
	At           time.Time   `json:"at"`
	Request      *Reference  `json:"correlation,omitempty"`
	Original     *Reference  `json:"original,omitempty"`
	Parent       EventID     `json:"parent,omitempty"`
	Phase        Phase       `json:"phase"`
	Reason       Reason      `json:"reason,omitempty"`
	Caller       string      `json:"caller,omitempty"`
	Effective    string      `json:"effective,omitempty"`
	Backend      string      `json:"backend,omitempty"`
	Origin       string      `json:"origin,omitempty"`
	Policy       string      `json:"policy,omitempty"`
	Profile      string      `json:"profile,omitempty"`
	Operation    string      `json:"operation,omitempty"`
	Variant      string      `json:"variant,omitempty"`
	Risk         uint8       `json:"risk,omitempty"`
	Requirements []string    `json:"requirements,omitempty"`
	Operands     []Operand   `json:"operands,omitempty"`
	DurationNS   int64       `json:"duration_ns,omitempty"`
	Counters     *Counters   `json:"counters,omitempty"`
	Losses       *LossCounts `json:"losses,omitempty"`
}

var ErrInvalidEvent = errors.New("invalid audit event")

func eventID(namespace string, sequence uint64) EventID {
	return EventID(namespace + ":" + strconv.FormatUint(sequence, 10))
}

func validEventID(id EventID) bool {
	prefix, sequence, ok := strings.Cut(string(id), ":")
	if !ok || !label(prefix) || len(sequence) > 20 {
		return false
	}
	count, err := strconv.ParseUint(sequence, 10, 64)
	return err == nil && count != 0 && string(id) == string(eventID(prefix, count))
}

func reference(r Reference) bool {
	return label(r.Runtime) && r.Request != 0
}

func label(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, ch := range s {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9',
			ch == '_', ch == '-', ch == '.':
		default:
			return false
		}
	}
	return true
}

func optionalLabel(s string) string {
	if label(s) {
		return s
	}
	return ""
}

func relative(s string) string {
	if s == "" || len(s) > 768 || !utf8.ValidString(s) || strings.ContainsAny(s, `\:`) ||
		path.IsAbs(s) || path.Clean(s) != s || s == ".." || strings.HasPrefix(s, "../") {
		return ""
	}
	for _, ch := range s {
		if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) || ch == '\u2028' || ch == '\u2029' {
			return ""
		}
	}
	if diagnostics.Redact(s) != s {
		return ""
	}
	for _, part := range strings.Split(strings.ToLower(s), "/") {
		base, _, _ := strings.Cut(strings.TrimPrefix(part, "."), ".")
		switch base {
		case "env", "credentials", "credential", "secrets", "secret", "password", "passwords",
			"token", "tokens", "id_rsa", "id_ed25519":
			return ""
		}
		if strings.HasSuffix(part, ".key") || strings.HasSuffix(part, ".pem") {
			return ""
		}
	}
	return s
}

func identity(key []byte, principal string) string {
	if len(key) == 0 || principal == "" || len(principal) > 512 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(principal))
	return hex.EncodeToString(mac.Sum(nil))
}

func checkedReason(r Reason) bool {
	switch r {
	case Allowed, Denied, RiskDenied, GrantDenied, RootDenied, NativeDenied, Invalid, Unsupported,
		Mismatch, Completed, Failed, Cancelled, TimedOut, Unavailable, Uncertain:
		return true
	default:
		return false
	}
}

func project(observation Observation, id EventID, at time.Time, key []byte) (event, error) {
	switch observation.Phase {
	case Decision, Intent, Outcome, Progress:
	default:
		return event{}, ErrInvalidEvent
	}
	if !reference(observation.Request) || !checkedReason(observation.Reason) ||
		observation.Parent != "" && (!validEventID(observation.Parent) || observation.Parent == id) ||
		observation.Original != nil && (!reference(*observation.Original) || *observation.Original == observation.Request) ||
		len(observation.Operands) > 8 || len(observation.Requirements) > 16 || observation.Risk > 3 {
		return event{}, ErrInvalidEvent
	}
	e := event{
		Version: 1, ID: id, At: at.UTC(), Request: &observation.Request,
		Original: observation.Original, Parent: observation.Parent, Phase: observation.Phase, Reason: observation.Reason,
		Caller: identity(key, observation.Caller), Effective: identity(key, observation.Effective),
		Backend: optionalLabel(observation.Backend), Origin: optionalLabel(observation.Origin),
		Policy: optionalLabel(observation.Policy), Profile: optionalLabel(observation.Profile),
		Operation: optionalLabel(observation.Operation), Variant: optionalLabel(observation.Variant),
		Risk: observation.Risk, DurationNS: max(0, observation.Elapsed.Nanoseconds()),
	}
	if observation.Counters != (Counters{}) {
		e.Counters = &observation.Counters
	}
	for _, requirement := range observation.Requirements {
		if !label(requirement) {
			return event{}, ErrInvalidEvent
		}
		e.Requirements = append(e.Requirements, requirement)
	}
	for _, operand := range observation.Operands {
		if !label(operand.Kind) || !label(operand.Role) || !label(operand.ID) {
			return event{}, ErrInvalidEvent
		}
		operand.Relative = relative(operand.Relative)
		e.Operands = append(e.Operands, operand)
	}
	return e, nil
}

func encode(e event, limit int) ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil || len(data) >= limit {
		return nil, ErrInvalidEvent
	}
	return append(data, '\n'), nil
}

func priority(o Observation) Priority {
	switch {
	case o.Reason == Mismatch || o.Reason == Uncertain:
		return Critical
	case o.Risk >= 3 || o.Phase == Decision && o.Reason != Allowed:
		return High
	case o.Risk == 2 || o.Phase == Outcome && o.Reason != Completed:
		return Normal
	default:
		return Low
	}
}
