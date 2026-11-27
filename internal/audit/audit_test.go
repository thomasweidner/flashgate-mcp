package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func recorderLimits() Limits {
	return Limits{EventBytes: 1024, QueueEvents: 8, QueueBytes: 8 << 10,
		BatchEvents: 4, BatchBytes: 4 << 10, FlushInterval: 5 * time.Millisecond,
		Shutdown: time.Second, IntentTimeout: 200 * time.Millisecond}
}

func TestProjectionAndNamespace(t *testing.T) {
	request := Reference{"runtimeA", 23}
	original := Reference{"runtimeB", 19}
	o := Observation{
		Request: request, Original: &original, Parent: eventID("runtimeB", 9),
		Phase: Outcome, Reason: Completed, Caller: "caller-identity", Effective: "backend-identity",
		Backend: "native", Operation: "unknown.synthetic", Variant: "default",
		Operands: []Operand{
			{Kind: "file", Role: "source", ID: "rootA", Relative: "quarterly/plan.txt"},
			{Kind: "file", Role: "target", ID: "rootB", Relative: "malicious\u2028name\r\n"},
		},
		Counters: Counters{ReadBytes: new(uint64(40))},
	}
	e, err := project(o, eventID("runtimeA", 1), time.Now(), []byte("instance-specific-key"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := encode(e, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if e.Caller == "" || e.Effective == "" || e.Caller == e.Effective ||
		bytes.Contains(b, []byte("caller-identity")) || bytes.Contains(b, []byte("backend-identity")) ||
		bytes.Contains(b, []byte("malicious")) || bytes.Count(b, []byte{'\n'}) != 1 ||
		e.Operands[0].Relative != "quarterly/plan.txt" || e.Operands[1].Relative != "" ||
		e.Parent != eventID("runtimeB", 9) || e.Counters.ReadBytes == nil || *e.Counters.ReadBytes != 40 {
		t.Fatalf("unsafe or incomplete projection: %+v", e)
	}
	for _, v := range []string{"../file", "/host/path", "C:/path", "path\\leaf", "password=unsafe"} {
		if relative(v) != "" {
			t.Fatalf("accepted unsafe path %q", v)
		}
	}
	o.Parent = eventID("runtimeA", 1)
	if _, err := project(o, o.Parent, time.Now(), []byte("key")); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("self-parent: %v", err)
	}
}

func TestQueuePriorityAndSize(t *testing.T) {
	recorder := &Recorder{limits: Limits{QueueBytes: 8, QueueEvents: 4}}
	entry := func(p Priority, letter byte, size int) record {
		return record{priority: p, data: bytes.Repeat([]byte{letter}, size)}
	}
	add := func(rec record) {
		t.Helper()
		if !recorder.enqueue(rec) {
			t.Fatal("unexpected queue rejection")
		}
	}
	add(entry(Critical, 'a', 2))
	add(entry(Low, 'b', 2))
	add(entry(Low, 'c', 2))
	add(entry(High, 'd', 2))
	add(entry(Normal, 'e', 4))
	if recorder.lost[Low][pressure] != 2 || recorder.bytes != 8 ||
		recorder.enqueue(entry(Low, 'f', 1)) {
		t.Fatal("priority/byte invariant violated")
	}
	recorder = &Recorder{limits: Limits{QueueBytes: 8, QueueEvents: 2}}
	add(entry(Critical, 'a', 4))
	add(entry(Critical, 'b', 4))
	add(entry(Critical, 'c', 4))
	if recorder.queue[0].data[0] != 'b' || recorder.lost[Critical][pressure] != 1 {
		t.Fatal("P0 must displace the least recent equivalent")
	}
	recorder.queue[0].intent = context.Background()
	add(entry(Critical, 'd', 4))
	if recorder.queue[0].data[0] != 'b' || recorder.queue[1].data[0] != 'd' {
		t.Fatal("durable intent was not protected")
	}
}

type captureSink struct {
	mu          sync.Mutex
	records     [][]byte
	durable     int
	unavailable bool
}

func (s *captureSink) Append(_ context.Context, records [][]byte, durable bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return ErrUnavailable
	}
	if durable {
		s.durable++
	}
	s.records = append(s.records, records...)
	return nil
}
func (s *captureSink) Maintain(context.Context) error { return nil }
func (s *captureSink) Close() error                   { return nil }

func TestRecorderDurableIntentAndDegradedOperation(t *testing.T) {
	sink := &captureSink{}
	recorder, err := NewRecorder(recorderLimits(), "runtimeA", [32]byte{42}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	o := Observation{Request: Reference{"runtimeA", 1}, Phase: Decision, Reason: Allowed,
		Operation: "synthetic", Variant: "default"}
	parent := recorder.Record(o)
	o.Phase, o.Parent = Intent, parent
	required, err := recorder.Require(ctx, o)
	if err != nil || required == "" || required == parent {
		t.Fatalf("intent %q: %v", required, err)
	}
	sink.mu.Lock()
	if sink.durable != 1 {
		t.Fatal("intent acknowledged before sync")
	}
	sink.unavailable = true
	sink.mu.Unlock()
	o.Request.Request++
	o.Parent = ""
	if _, err := recorder.Require(ctx, o); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("sink failure: %v", err)
	}
	o.Phase, o.Reason = Outcome, Completed
	if id := recorder.Record(o); id == "" {
		t.Fatal("ordinary runtime observation could not enter queue")
	}
	if err := recorder.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if h := recorder.Health(); h.Available || h.Lost[Low][writeFailure] < 2 {
		t.Fatalf("expected audited write loss: %+v", h)
	}
}

func TestRecorderFailClosedProjectionAndRecoveryNotice(t *testing.T) {
	sink := &captureSink{}
	var inactive *Recorder
	if inactive.Record(Observation{}) != "" || inactive.Close(t.Context()) != nil {
		t.Fatal("inactive audit refused optional recording")
	}
	if _, err := inactive.Require(t.Context(), Observation{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("inactive recorder confirmed intent: %v", err)
	}
	if _, err := NewRecorder(Limits{}, "scope", [32]byte{2}, sink, nil); !errors.Is(err, ErrLimits) {
		t.Fatalf("unbounded recorder accepted: %v", err)
	}
	bounds := recorderLimits()
	bounds.IntentTimeout = 5 * time.Millisecond
	r, err := NewRecorder(bounds, "scope", [32]byte{2}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	required := Observation{
		Request: Reference{"scope", 5}, Phase: Intent, Reason: Allowed, Risk: 3,
		Requirements: []string{"path.read"},
	}
	for _, bad := range []Observation{
		{Phase: "spoofed", Reason: Allowed},
		{Request: required.Request, Phase: Decision, Reason: "spoofed"},
		{Request: required.Request, Phase: Outcome, Reason: Completed, Parent: "bad-parent"},
		{Request: required.Request, Phase: Outcome, Reason: Completed, Requirements: []string{"bad requirement"}},
		{Request: required.Request, Phase: Outcome, Reason: Completed, Operands: []Operand{{ID: "bad id"}}},
	} {
		if r.Record(bad) != "" {
			t.Fatal("mandatory audit projection failed open")
		}
	}
	big := required
	big.Operands = []Operand{
		{Kind: "file", Role: "source", ID: "first", Relative: strings.Repeat("a", 600)},
		{Kind: "file", Role: "target", ID: "second", Relative: strings.Repeat("b", 600)},
	}
	if _, err := r.Require(t.Context(), big); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("oversized durable intent was admitted: %v", err)
	}
	if _, err := r.Require(t.Context(), required); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("intent acknowledged without writer: %v", err)
	}
	if batch := r.take(nil); len(batch) != 0 {
		t.Fatal("expired intent reached the writer")
	}
	r.Start()
	r.Record(Observation{Request: Reference{"scope", 6}, Phase: Outcome, Reason: Completed})
	if err := r.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r.Record(required) != "" {
		t.Fatal("closed recorder accepted a new event")
	}
	if r.Health().Lost[High][intentTimeout] != 1 || r.Health().Lost[High][shutdown] != 1 {
		t.Fatalf("expected bounded loss causes: %+v", r.Health())
	}
	summary := false
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for _, line := range sink.records {
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		if e.Phase == Loss && e.Losses != nil && e.Losses[High][intentTimeout] == 1 {
			summary = true
		}
	}
	if !summary {
		t.Fatal("available sink did not report observed loss")
	}
}

type stalledSink struct {
	started chan struct{}
	release chan struct{}
}

func (s *stalledSink) Append(context.Context, [][]byte, bool) error {
	close(s.started)
	<-s.release
	return nil
}
func (*stalledSink) Maintain(context.Context) error { return nil }
func (*stalledSink) Close() error                   { return nil }

func TestRecorderRejectsCanceledInFlightIntent(t *testing.T) {
	sink := &stalledSink{started: make(chan struct{}), release: make(chan struct{})}
	defer func() {
		select {
		case <-sink.release:
		default:
			close(sink.release)
		}
	}()
	limits := recorderLimits()
	limits.IntentTimeout = 5 * time.Second
	r, err := NewRecorder(limits, "running", [32]byte{1}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Start()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := r.Require(ctx, Observation{Request: Reference{"running", 1}, Phase: Intent, Reason: Allowed})
		result <- err
	}()
	select {
	case <-sink.started:
	case <-time.After(limits.IntentTimeout):
		t.Fatal("required intent did not reach the sink")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("canceled intent returned %v", err)
		}
	case <-time.After(limits.IntentTimeout):
		t.Fatal("required intent waited for the stalled sink after cancellation")
	}
	// A late successful append must leave the completed refusal in force.
	close(sink.release)
	if err := r.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !r.Health().Available {
		t.Fatal("the late append should leave the store available")
	}
}

func TestRecorderBoundsShutdownWaitDuringNativeIO(t *testing.T) {
	sink := &stalledSink{started: make(chan struct{}), release: make(chan struct{})}
	defer func() {
		select {
		case <-sink.release:
		default:
			close(sink.release)
		}
	}()
	limits := recorderLimits()
	limits.BatchEvents = 1
	r, err := NewRecorder(limits, "running", [32]byte{1}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Start()
	r.Record(Observation{Request: Reference{"running", 1}, Phase: Outcome, Reason: Completed})
	select {
	case <-sink.started:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not reach the stalled sink")
	}
	wait, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	if err := r.Close(wait); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("uninterruptible sink overrode host deadline: %v", err)
	}
	close(sink.release)
	if err := r.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestFailedLossNoticeRetainsCountersForRecovery(t *testing.T) {
	sink := &captureSink{unavailable: true}
	r, err := NewRecorder(recorderLimits(), "recovery", [32]byte{1}, sink, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(t.Context()) }()
	r.addLoss(Critical, pressure)
	r.setHealth(true, false)
	r.writeLoss(t.Context())
	if r.Health().Available || r.pending[Critical][pressure] != 1 {
		t.Fatal("failed notice cleared the pending loss record")
	}
	sink.unavailable = false
	r.writeLoss(t.Context())
	if r.pending[Critical][pressure] != 0 || r.Health().Lost[Critical][pressure] != 1 {
		t.Fatal("reported loss lost its cumulative health count")
	}
}

func TestProtectedIntentRefusalAndShutdownAccounting(t *testing.T) {
	limits := recorderLimits()
	limits.QueueEvents, limits.BatchEvents = 1, 1
	r, err := NewRecorder(limits, "queue", [32]byte{1}, &captureSink{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(t.Context()) }()
	ack := make(chan error, 1)
	intent := Observation{Request: Reference{"queue", 1}, Phase: Intent, Reason: Allowed}
	if id := r.record(intent, t.Context(), ack); id == "" {
		t.Fatal("protected fixture intent rejected")
	}
	if id := r.Record(Observation{Request: Reference{"queue", 2}, Phase: Outcome, Reason: Completed}); id != "" {
		t.Fatal("new record displaced the protected intent")
	}
	r.discard()
	if err := <-ack; !errors.Is(err, ErrUnavailable) {
		t.Fatalf("undelivered intent got an invalid shutdown result: %v", err)
	}
	health := r.Health()
	if health.Lost[Low][pressure] != 1 || health.Lost[Low][shutdown] != 1 {
		t.Fatalf("lost pressure or shutdown accounting: %+v", health)
	}
}

func TestFileStoreRotationAndSharedLimit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{Bytes: 12 << 10, SegmentBytes: 4096, Retention: 24 * time.Hour}
	left, err := NewFileStore(root, "server", limits)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewFileStore(root, "server", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	defer right.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := left.Maintain(ctx); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	for writer, store := range []*FileStore{left, right} {
		go func() {
			for i := range 100 {
				value := []byte("{\"writer\":" + strconv.Itoa(writer) + ",\"sequence\":" + strconv.Itoa(i) +
					",\"data\":\"" + string(bytes.Repeat([]byte{'a'}, 384)) + "\"}\n")
				if err := store.Append(ctx, [][]byte{value}, false); err != nil {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	var total int64
	segments := 0
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		segments++
		if info.Size() > limits.SegmentBytes {
			t.Errorf("segment exceeds bound: %d", info.Size())
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if !json.Valid(scanner.Bytes()) {
				t.Error("corrupted JSONL record")
			}
		}
		return scanner.Err()
	}); err != nil {
		t.Fatal(err)
	}
	if total > limits.Bytes || segments < 2 {
		t.Fatalf("unexpected retained files: bytes=%d, segments=%d", total, segments)
	}
	mismatch, err := NewFileStore(root, "other-server", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer mismatch.Close()
	if err := mismatch.Maintain(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("wrong store binding: %v", err)
	}
}
