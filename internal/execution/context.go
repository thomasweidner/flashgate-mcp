// Package execution binds trusted requests to an effective policy and a
// selected execution backend. It does not know about MCP tools or native I/O.
package execution

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

var (
	ErrDenied      = errors.New("execution denied")
	ErrUnsupported = errors.New("execution backend unavailable or unsupported")
	ErrMismatch    = errors.New("execution binding mismatch")
)

type Origin string
type PrincipalID string
type GroupID string
type BackendID string
type Protocol string

const (
	InheritedSTDIO Origin    = "inherited-stdio"
	CurrentProcess BackendID = "current-process"
	ServiceAccount BackendID = "service-account"
	UserWorker     BackendID = "user-worker"
	MCP20251125    Protocol  = "2025-11-25"
	MCP20260728    Protocol  = "2026-07-28"
)

// Caller is the authenticated principal as seen by the trusted ingress.
// Groups are attributes for policy evaluation. Their presence grants no rights.
type Caller struct {
	id     PrincipalID
	origin Origin
	groups []GroupID
}

func (c Caller) ID() PrincipalID   { return c.id }
func (c Caller) Origin() Origin    { return c.origin }
func (c Caller) Groups() []GroupID { return slices.Clone(c.groups) }

// Identity describes the account actually used by an execution backend.
// Binding distinguishes process, token or worker instances of the same account.
type Identity struct {
	Account PrincipalID
	Binding string
}

func (i Identity) valid() bool {
	return strings.TrimSpace(string(i.Account)) != "" && strings.TrimSpace(i.Binding) != ""
}

// Runtime represents one immutable policy/configuration generation. A new
// instance is required when the effective configuration is replaced.
type Runtime struct {
	generation string
	next       atomic.Uint64
}

func NewRuntime(generation string) (*Runtime, error) {
	if strings.TrimSpace(generation) == "" {
		return nil, ErrMismatch
	}
	return &Runtime{generation: generation}, nil
}

// Session accepts requests from one trusted ingress, under one runtime.
type Session struct {
	runtime *Runtime
	caller  Caller
	policy  *PolicyBinding
}

// PolicyBinding is an immutable policy selection in one runtime. The trusted
// ingress assigns it after authentication. Equal grants do not identify a binding.
type PolicyBinding struct {
	runtime *Runtime
	profile config.EffectiveProfile
}

func NewPolicyBinding(runtime *Runtime, profile config.EffectiveProfile) (*PolicyBinding, error) {
	if runtime == nil || runtime.generation == "" || profile.ID() == "" {
		return nil, ErrMismatch
	}
	return &PolicyBinding{runtime: runtime, profile: profile}, nil
}

func (s Session) WithPolicy(policy *PolicyBinding) (Session, error) {
	if policy == nil || policy.runtime == nil || policy.runtime != s.runtime || s.policy != nil || s.caller.ID() == "" {
		return Session{}, ErrMismatch
	}
	s.policy = policy
	return s, nil
}

func newSession(runtime *Runtime, id PrincipalID, origin Origin, groups []GroupID) (Session, error) {
	if runtime == nil || runtime.generation == "" || strings.TrimSpace(string(id)) == "" || origin != InheritedSTDIO {
		return Session{}, ErrMismatch
	}
	groups = slices.Clone(groups)
	for _, group := range groups {
		if strings.TrimSpace(string(group)) == "" {
			return Session{}, ErrMismatch
		}
	}
	slices.Sort(groups)
	return Session{runtime: runtime, caller: Caller{id: id, origin: origin, groups: slices.Compact(groups)}}, nil
}

// BindDirectSTDIO derives the direct caller and backend account from the
// inherited process credentials. It does not authenticate a user in a payload.
func BindDirectSTDIO(runtime *Runtime) (Session, Identity, error) {
	identity, groups, err := currentProcess(true)
	if err != nil {
		return Session{}, Identity{}, ErrUnsupported
	}
	session, err := newSession(runtime, identity.Account, InheritedSTDIO, groups)
	return session, identity, err
}

// CheckCurrentProcess verifies the identity binding at native dispatch.
func CheckCurrentProcess(expected Identity) error {
	identity, _, err := currentProcess(false)
	if err != nil || !expected.valid() || identity != expected {
		return ErrMismatch
	}
	return nil
}

// Request is immutable. Its server-assigned number is unique in the runtime
// and also serves as the correlation ID. The wire ID is only for MCP framing.
type Request struct {
	session     Session
	correlation uint64
	protocol    Protocol
	resultBytes int64
	invocation  *catalogInvocationBinding
}

func (s Session) Request(protocol Protocol, resultBytes int64) (Request, error) {
	if s.runtime == nil || (protocol != MCP20251125 && protocol != MCP20260728) {
		return Request{}, ErrMismatch
	}
	return Request{session: s, correlation: s.runtime.next.Add(1), protocol: protocol, resultBytes: resultBytes}, nil
}

func (r Request) Caller() Caller      { return r.session.caller }
func (r Request) Correlation() uint64 { return r.correlation }
func (r Request) Protocol() Protocol  { return r.protocol }
func (r Request) ResultBytes() int64  { return r.resultBytes }
