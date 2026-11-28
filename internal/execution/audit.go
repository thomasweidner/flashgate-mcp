package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/audit"
	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type Auditing struct {
	recorder *audit.Recorder
	required map[OperationKey]bool
}

// BindAuditing accepts declared names, exact variants, or a supported class.
// A name selects all variants. Configuration does not confer operation rights.
func BindAuditing(recorder *audit.Recorder, selected []string, declarations []Declaration) (*Auditing, error) {
	if recorder == nil {
		if len(selected) > 0 {
			return nil, ErrMismatch
		}
		return nil, nil
	}
	required := make(map[OperationKey]bool)
	for _, declaration := range declarations {
		if declaration.DurableIntent {
			required[declaration.Key] = true
		}
	}
	for _, selection := range selected {
		matched := false
		for _, declaration := range declarations {
			key := declaration.Key
			if selection == string(key.Name) || selection == string(key.Name)+":"+string(key.Variant) ||
				selection == riskSelection(declaration.Operation.Risk()) {
				matched, required[key] = true, true
			}
		}
		if !matched {
			return nil, ErrMismatch
		}
	}
	return &Auditing{recorder, required}, nil
}

func riskSelection(risk capability.Risk) string {
	switch risk {
	case capability.RiskRead:
		return "risk:read"
	case capability.RiskMutation:
		return "risk:mutation"
	case capability.RiskHigh:
		return "risk:high"
	default:
		return ""
	}
}

type auditSpan struct {
	recorder *audit.Recorder
	required bool
	base     audit.Observation
	once     sync.Once
	failed   error
	at       time.Time
	paths    []audit.Operand
}

// RecordCatalogDenial records an opaque denial for an authenticated request.
// An unadmitted tool name is never taken from the client or the master catalog.
func (c CatalogContext) RecordCatalogDenial() {
	if !c.Valid() {
		return
	}
	route, _ := c.composition.Route("")
	auditIngress(route.binding, c.request, audit.Denied)
}

func (i CatalogInvocation) RecordInvalidArguments() {
	request := i.request
	if request.invocation == nil || request.invocation.attempted.Load() {
		return
	}
	auditIngress(request.invocation.route.binding, request, audit.Invalid)
}

func auditIngress(binding *dispatchBinding, request Request, reason audit.Reason) {
	if binding == nil || binding.auditing == nil {
		return
	}
	binding.auditing.recorder.Record(audit.Observation{
		Request: audit.Reference{Runtime: request.session.runtime.generation, Request: request.Correlation()},
		Phase:   audit.Decision, Reason: reason, Caller: string(request.Caller().ID()),
		Origin: string(request.Caller().Origin()), Policy: string(binding.id), Profile: string(binding.policy.profile.ID()),
	})
}

func (a *Auditing) decision(binding *dispatchBinding, request Request, declaration Declaration, resources []BoundResource, failure error) *auditSpan {
	if a == nil {
		return nil
	}
	observation := audit.Observation{
		Request: audit.Reference{Runtime: request.session.runtime.generation, Request: request.Correlation()},
		Phase:   audit.Decision, Reason: auditReason(failure, true), Caller: string(request.Caller().ID()),
		Origin: string(request.Caller().Origin()), Policy: string(binding.id), Profile: string(binding.policy.profile.ID()),
		Operation: string(declaration.Key.Name), Variant: string(declaration.Key.Variant),
		Risk: uint8(declaration.Operation.Risk()),
	}
	for _, id := range declaration.Operation.Requirement().IDs() {
		observation.Requirements = append(observation.Requirements, string(id))
	}
	for _, operand := range resources {
		observation.Operands = append(observation.Operands,
			audit.Operand{Kind: string(operand.Kind), ID: operand.ID, Role: string(operand.Role)})
	}
	id := a.recorder.Record(observation)
	if failure != nil {
		return nil
	}
	observation.Parent = id
	paths := slices.Clone(observation.Operands)
	for i, resource := range resources {
		paths[i].Relative = filepath.ToSlash(resource.Relative)
	}
	return &auditSpan{recorder: a.recorder, required: a.required[declaration.Key], base: observation, at: time.Now(), paths: paths}
}

func (s *auditSpan) start(ctx context.Context, binding *Binding) error {
	if s == nil {
		return nil
	}
	s.once.Do(func() { s.recordIntent(ctx, binding) })
	return s.failed
}

func (s *auditSpan) recordIntent(ctx context.Context, binding *Binding) {
	intent := s.base
	intent.Phase = audit.Intent
	intent.Operands = s.paths
	intent.Backend = string(binding.Backend())
	intent.Effective = string(binding.EffectiveIdentity().Account)
	var id audit.EventID
	if s.required {
		id, s.failed = s.recorder.Require(ctx, intent)
	} else {
		id = s.recorder.Record(intent)
	}
	if id != "" {
		s.base.Parent = id
	}
	if s.failed == nil {
		s.base.Operands, s.base.Backend, s.base.Effective = s.paths, intent.Backend, intent.Effective
	}
}

func (s *auditSpan) finish(reason audit.Reason, counters audit.Counters) {
	s.base.Phase, s.base.Reason = audit.Outcome, reason
	s.base.Elapsed, s.base.Counters = time.Since(s.at), counters
	s.recorder.Record(s.base)
}

func auditReason(err error, decision bool) audit.Reason {
	var policy policyDenial
	switch {
	case err == nil && decision:
		return audit.Allowed
	case err == nil:
		return audit.Completed
	case errors.Is(err, context.Canceled):
		return audit.Cancelled
	case errors.Is(err, context.DeadlineExceeded):
		return audit.TimedOut
	case errors.Is(err, audit.ErrUnavailable):
		return audit.Unavailable
	case errors.As(err, &policy):
		if policy.risk {
			return audit.RiskDenied
		}
		return audit.GrantDenied
	case errors.Is(err, ErrDenied):
		return audit.RootDenied
	case errors.Is(err, os.ErrPermission), errors.Is(err, security.ErrConfigurationAccess):
		return audit.NativeDenied
	case errors.Is(err, ErrMismatch):
		return audit.Mismatch
	case errors.Is(err, ErrUnsupported):
		return audit.Unsupported
	default:
		return audit.Failed
	}
}
