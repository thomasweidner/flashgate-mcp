package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/audit"
	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type auditProbeSink struct {
	records [][]byte
	durable int
}

func TestAuditUsesNormalizedDecisionAndOutcomeReasons(t *testing.T) {
	for _, tc := range []struct {
		err      error
		decision bool
		want     audit.Reason
	}{
		{nil, true, audit.Allowed},
		{nil, false, audit.Completed},
		{context.Canceled, false, audit.Cancelled},
		{context.DeadlineExceeded, false, audit.TimedOut},
		{audit.ErrUnavailable, false, audit.Unavailable},
		{policyDenial{risk: true}, true, audit.RiskDenied},
		{policyDenial{}, true, audit.GrantDenied},
		{ErrDenied, true, audit.RootDenied},
		{os.ErrPermission, false, audit.NativeDenied},
		{security.ErrConfigurationAccess, false, audit.NativeDenied},
		{ErrMismatch, false, audit.Mismatch},
		{ErrUnsupported, false, audit.Unsupported},
		{errors.New("raw backend failure"), false, audit.Failed},
	} {
		if actual := auditReason(tc.err, tc.decision); actual != tc.want {
			t.Fatalf("audit reason = %q, want %q", actual, tc.want)
		}
	}
	for risk, selection := range map[capability.Risk]string{
		capability.RiskRead: "risk:read", capability.RiskMutation: "risk:mutation",
		capability.RiskHigh: "risk:high", 0: "",
	} {
		if got := riskSelection(risk); got != selection {
			t.Fatalf("selector = %q, want %q", got, selection)
		}
	}
	if _, err := BindAuditing(nil, []string{"fixture"}, nil); !errors.Is(err, ErrMismatch) {
		t.Fatalf("required selection accepted with auditing disabled: %v", err)
	}
}

func (s *auditProbeSink) Append(_ context.Context, records [][]byte, durable bool) error {
	for _, data := range records {
		s.records = append(s.records, append([]byte(nil), data...))
	}
	if durable {
		s.durable++
	}
	return nil
}
func (*auditProbeSink) Maintain(context.Context) error { return nil }
func (*auditProbeSink) Close() error                   { return nil }

func TestPanickingBackendRecordsUncertainOutcome(t *testing.T) {
	for _, admit := range []bool{false, true} {
		name := "before admission"
		if admit {
			name = "after admission"
		}
		t.Run(name, func(t *testing.T) {
			_, session, _ := sessionFixture(t, nil)
			profile := profileFixture(t, "active='inspector'\n[profiles.inspector]\ngrants=['system.read']\n")
			decl := fixtureDeclaration(t, profile, capability.RiskRead, capability.SystemRead)
			decl.DurableIntent = true
			routes := fixtureRoutes(decl)
			sink := &auditProbeSink{}
			recorder, err := audit.NewRecorder(audit.Limits{
				EventBytes: 4096, QueueEvents: 16, QueueBytes: 64 << 10, BatchEvents: 8, BatchBytes: 32 << 10,
				FlushInterval: time.Millisecond, Shutdown: time.Second, IntentTimeout: time.Second,
			}, "panic-fixture", [32]byte{17}, sink, nil)
			if err != nil {
				t.Fatal(err)
			}
			recorder.Start()
			t.Cleanup(func() { _ = recorder.Close(context.Background()) })
			routes.Audit, err = BindAuditing(recorder, nil, routes.Declarations)
			if err != nil {
				t.Fatal(err)
			}
			const panicValue = "fixture-panic-payload"
			var calls int
			backend := probeBackend(func(ctx context.Context, call Invocation[probe]) (Completion[string], error) {
				calls++
				if admit {
					if err := call.AdmitEffect(ctx); err != nil {
						return Completion[string]{}, err
					}
				}
				panic(panicValue)
			})
			policy, request := fixturePolicy(t, session, profile)
			dispatcher, err := NewDispatcher(policy, ServiceAccount, routes, Registration[probe, string]{
				ID: ServiceAccount, Identity: Identity{Account: "simulated-native-account", Binding: "native-worker"},
				Backend: backend, Operations: routes.OperationKeys(),
			})
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if caught := recover(); caught != panicValue {
						t.Errorf("expected original panic, got %v", caught)
					}
				}()
				_, _ = dispatcher.Dispatch(t.Context(), request, probe{}, nil)
			}()
			if err := recorder.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
			intentCount := 0
			if admit {
				intentCount = 1
			}
			if calls != 1 || len(sink.records) != 2+intentCount {
				t.Fatalf("unexpected calls or records: %d / %d", calls, len(sink.records))
			}
			outcome := sink.records[len(sink.records)-1]
			var event struct{ Phase, Reason string }
			if err := json.Unmarshal(outcome, &event); err != nil || event.Phase != "outcome" ||
				event.Reason != "uncertain" || strings.Contains(string(outcome), panicValue) {
				t.Fatalf("incorrect panic outcome: %s (%v)", outcome, err)
			}
		})
	}
}

func TestAuditKeepsObservedBytesWhenCompletionIsRejected(t *testing.T) {
	_, session, _ := sessionFixture(t, nil)
	profile := profileFixture(t, "active='inspector'\n[profiles.inspector]\ngrants=['system.read']\n")
	routes := fixtureRoutes(fixtureDeclaration(t, profile, capability.RiskRead, capability.SystemRead))
	sink := &auditProbeSink{}
	recorder, err := audit.NewRecorder(audit.Limits{
		EventBytes: 4096, QueueEvents: 16, QueueBytes: 64 << 10, BatchEvents: 8, BatchBytes: 32 << 10,
		FlushInterval: time.Millisecond, Shutdown: time.Second, IntentTimeout: time.Second,
	}, "rejected-result", [32]byte{17}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder.Start()
	t.Cleanup(func() { _ = recorder.Close(context.Background()) })
	routes.Audit, err = BindAuditing(recorder, nil, routes.Declarations)
	if err != nil {
		t.Fatal(err)
	}
	policy, request := fixturePolicy(t, session, profile)
	dispatcher, err := NewDispatcher(policy, ServiceAccount, routes, Registration[probe, string]{
		ID: ServiceAccount, Identity: Identity{Account: "simulated-native-account", Binding: "native-worker"},
		Backend: probeBackend(func(_ context.Context, call Invocation[probe]) (Completion[string], error) {
			return Complete(call.Binding(), "read"), nil
		}),
		Operations: routes.OperationKeys(),
		Counters: func(_ probe, value string) audit.Counters {
			return audit.Counters{ReadBytes: new(uint64(len(value)))}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	got, err := dispatcher.Dispatch(ObserveOperations(ctx, func(OperationKey) { cancel() }), request, probe{}, nil)
	if got != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("rejected completion: %q (%v)", got, err)
	}
	if err := recorder.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	var outcome struct {
		Reason   string
		Counters audit.Counters
	}
	if len(sink.records) != 2 {
		t.Fatalf("unexpected record count: %d", len(sink.records))
	}
	if err := json.Unmarshal(sink.records[1], &outcome); err != nil || outcome.Reason != "cancelled" ||
		outcome.Counters.ReadBytes == nil || *outcome.Counters.ReadBytes != 4 || outcome.Counters.WrittenBytes != nil {
		t.Fatalf("wrong rejected outcome: %s (%v)", sink.records[1], err)
	}
}

func TestDeclaredSyntheticAuditAndLaterAuthorization(t *testing.T) {
	_, session, _ := sessionFixture(t, nil)
	allowed := profileFixture(t, "active='inspector'\n[profiles.inspector]\ngrants=['system.read']\n")
	denied := profileFixture(t, "")
	decl := fixtureDeclaration(t, allowed, capability.RiskRead, capability.SystemRead)
	decl.Key.Name = "future_job.resume"
	decl.DurableIntent = true
	sink := &auditProbeSink{}
	recorder, err := audit.NewRecorder(audit.Limits{
		EventBytes: 4096, QueueEvents: 16, QueueBytes: 64 << 10, BatchEvents: 8, BatchBytes: 32 << 10,
		FlushInterval: time.Millisecond, Shutdown: time.Second, IntentTimeout: time.Second,
	}, "trusted-generation", [32]byte{17}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder.Start()
	routes := fixtureRoutes(decl)
	var backendCalls int
	backend := probeBackend(func(ctx context.Context, call Invocation[probe]) (Completion[string], error) {
		backendCalls++
		if err := call.AdmitEffect(ctx); err != nil {
			return Completion[string]{}, err
		}
		if err := call.AdmitEffect(ctx); err != nil {
			return Completion[string]{}, err
		}
		return Complete(call.Binding(), "actual result"), nil
	})
	registration := Registration[probe, string]{
		ID: ServiceAccount, Identity: Identity{Account: "simulated-native-account", Binding: "native-worker"},
		Backend: backend, Operations: routes.OperationKeys(),
	}
	policy, request := fixturePolicy(t, session, allowed)
	if _, err := NewDispatcher(policy, ServiceAccount, routes, registration); !errors.Is(err, ErrMismatch) {
		t.Fatalf("mandatory intent accepted without auditing: %v", err)
	}
	routes.Audit, err = BindAuditing(recorder, nil, routes.Declarations)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher, err := NewDispatcher(policy, ServiceAccount, routes, registration)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := dispatcher.Dispatch(t.Context(), request, probe{}, nil); err != nil || got != "actual result" {
		t.Fatalf("synthetic operation = %q, %v", got, err)
	}
	laterPolicy, laterRequest := fixturePolicy(t, session, denied)
	laterDeclaration := fixtureDeclaration(t, denied, capability.RiskRead, capability.SystemRead)
	laterDeclaration.Key, laterDeclaration.DurableIntent = decl.Key, true
	laterRoutes := fixtureRoutes(laterDeclaration)
	laterRoutes.Audit = routes.Audit
	later, err := NewDispatcher(laterPolicy, ServiceAccount, laterRoutes, registration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := later.Dispatch(t.Context(), laterRequest, probe{}, nil); !errors.Is(err, ErrDenied) {
		t.Fatalf("later operation inherited authority: %v", err)
	}
	if err := recorder.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if backendCalls != 1 || sink.durable != 1 || len(sink.records) != 4 {
		t.Fatalf("duplicate or missing events, calls=%d durable=%d records=%d", backendCalls, sink.durable, len(sink.records))
	}
	var events [4]struct {
		ID, Parent, Caller, Effective, Phase, Reason string
		Correlation                                  audit.Reference
	}
	for index, data := range sink.records {
		if err := json.Unmarshal(data, &events[index]); err != nil {
			t.Fatal(err)
		}
	}
	decision, intent, outcome, rejection := events[0], events[1], events[2], events[3]
	if decision.Phase != "decision" || intent.Phase != "intent" || outcome.Phase != "outcome" ||
		intent.Parent != decision.ID || outcome.Parent != intent.ID ||
		intent.Caller == "" || intent.Effective == "" || intent.Effective == intent.Caller ||
		rejection.Phase != "decision" || rejection.Reason != "grant_denied" || rejection.Effective != "" ||
		rejection.Correlation.Request == decision.Correlation.Request {
		t.Fatalf("unsafe synthetic evidence: %+v", events)
	}
}
