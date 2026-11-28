package audit

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrUnavailable = errors.New("audit persistence unavailable")
	ErrLimits      = errors.New("invalid audit limits")
)

type Limits struct {
	EventBytes    int
	QueueEvents   int
	QueueBytes    int
	BatchEvents   int
	BatchBytes    int
	FlushInterval time.Duration
	Shutdown      time.Duration
	IntentTimeout time.Duration
}

func (l Limits) Valid() bool {
	return l.EventBytes >= 1024 && l.EventBytes <= 64<<10 &&
		l.QueueEvents >= 1 && l.QueueEvents <= 16384 &&
		l.BatchEvents >= 1 && l.BatchEvents <= l.QueueEvents &&
		l.BatchBytes >= l.EventBytes && l.BatchBytes <= l.QueueBytes &&
		l.QueueBytes <= 64<<20 && l.FlushInterval >= time.Millisecond &&
		l.FlushInterval <= time.Second && l.Shutdown >= l.FlushInterval &&
		l.Shutdown <= 10*time.Second && l.IntentTimeout >= time.Millisecond &&
		l.IntentTimeout <= 5*time.Second
}

// Sink has one caller. Append must return success only after the full batch
// was appended and, for a durable batch, confirmed by the native file system.
type Sink interface {
	Append(context.Context, [][]byte, bool) error
	Maintain(context.Context) error
	Close() error
}

type lossCause uint8

const (
	pressure lossCause = iota
	projection
	writeFailure
	shutdown
	intentTimeout
)

type LossCounts [priorityCount][5]uint64

type Health struct {
	Available bool
	Lost      LossCounts
}

type record struct {
	data     []byte
	at       time.Time
	priority Priority
	intent   context.Context
	ack      chan error
}

type Recorder struct {
	limits    Limits
	namespace string
	key       [32]byte
	sink      Sink
	report    func(Health)
	start     sync.Once
	sequence  atomic.Uint64
	mu        sync.Mutex
	queue     []record
	bytes     int
	lost      LossCounts
	pending   LossCounts
	healthy   bool
	closing   bool
	wake      chan struct{}
	stop      chan struct{}
	done      chan struct{}
}

func NewRecorder(limits Limits, namespace string, key [32]byte, sink Sink, report func(Health)) (*Recorder, error) {
	if !limits.Valid() || !label(namespace) || key == ([32]byte{}) || sink == nil {
		return nil, ErrLimits
	}
	r := &Recorder{
		limits: limits, namespace: namespace, key: key, sink: sink, report: report,
		queue: make([]record, 0, limits.QueueEvents),
		wake:  make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
	}
	return r, nil
}

// Start runs after the host has validated the full configuration and catalog.
func (r *Recorder) Start() {
	r.start.Do(func() { go r.run() })
}

func (r *Recorder) Record(o Observation) EventID {
	if r == nil {
		return ""
	}
	return r.record(o, nil, nil)
}

// Require waits for durable intent separately from the outcome. The caller
// must still revalidate its authority before starting the effect.
func (r *Recorder) Require(ctx context.Context, o Observation) (EventID, error) {
	if r == nil || ctx == nil || o.Phase != Intent {
		return "", ErrUnavailable
	}
	wait, cancel := context.WithTimeout(ctx, r.limits.IntentTimeout)
	defer cancel()
	ack := make(chan error, 1)
	id := r.record(o, wait, ack)
	if id == "" {
		return "", ErrUnavailable
	}
	select {
	case err := <-ack:
		if err == nil && wait.Err() == nil {
			return id, nil
		}
	case <-wait.Done():
	}
	return id, ErrUnavailable
}

func (r *Recorder) record(o Observation, intent context.Context, ack chan error) EventID {
	p := priority(o)
	id := eventID(r.namespace, r.sequence.Add(1))
	e, err := project(o, id, time.Now(), r.key[:])
	var data []byte
	if err == nil {
		data, err = encode(e, r.limits.EventBytes)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.addLoss(p, projection)
		return ""
	}
	if r.closing {
		r.addLoss(p, shutdown)
		return ""
	}
	if !r.enqueue(record{data: data, at: e.At, priority: p, intent: intent, ack: ack}) {
		r.addLoss(p, pressure)
		return ""
	}
	if intent != nil || len(r.queue) >= r.limits.BatchEvents || r.bytes >= r.limits.BatchBytes {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
	return id
}

// enqueue is called under mu. An ordinary event competes on priority and
// age. A durable intent is eligible to displace ordinary events of any class.
func (r *Recorder) enqueue(next record) bool {
	needCount := len(r.queue) + 1 - r.limits.QueueEvents
	needBytes := r.bytes + len(next.data) - r.limits.QueueBytes
	if needCount > 0 || needBytes > 0 {
		floor := next.priority
		if next.intent != nil {
			floor = Critical
		}
		var possibleCount, possibleBytes int
		for _, entry := range r.queue {
			if entry.intent == nil && entry.priority >= floor {
				possibleCount++
				possibleBytes += len(entry.data)
			}
		}
		if possibleCount < needCount || possibleBytes < needBytes {
			return false
		}
		for p := int(Low); p >= int(floor) && (needCount > 0 || needBytes > 0); p-- {
			for i := range r.queue {
				entry := &r.queue[i]
				if entry.intent != nil || entry.priority != Priority(p) {
					continue
				}
				n := len(entry.data)
				r.bytes -= n
				r.addLoss(entry.priority, pressure)
				entry.data = nil
				needCount--
				needBytes -= n
				if needCount <= 0 && needBytes <= 0 {
					break
				}
			}
		}
		n := 0
		for _, entry := range r.queue {
			if entry.data != nil {
				r.queue[n] = entry
				n++
			}
		}
		clear(r.queue[n:])
		r.queue = r.queue[:n]
	}
	r.queue = append(r.queue, next)
	r.bytes += len(next.data)
	return true
}

func increment(n *uint64) {
	if *n != math.MaxUint64 {
		*n++
	}
}

func (r *Recorder) addLoss(priority Priority, cause lossCause) {
	increment(&r.lost[priority][cause])
	increment(&r.pending[priority][cause])
}

func (r *Recorder) Health() Health {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Health{Available: r.healthy, Lost: r.lost}
}

func (r *Recorder) take(batch []record) []record {
	r.mu.Lock()
	defer r.mu.Unlock()
	var consumed, bytes int
	for _, entry := range r.queue {
		if entry.intent != nil && entry.intent.Err() != nil {
			r.addLoss(entry.priority, intentTimeout)
			consumed++
			r.bytes -= len(entry.data)
			continue
		}
		if len(batch) == r.limits.BatchEvents || bytes+len(entry.data) > r.limits.BatchBytes {
			break
		}
		batch = append(batch, entry)
		consumed++
		bytes += len(entry.data)
		r.bytes -= len(entry.data)
	}
	remaining := copy(r.queue, r.queue[consumed:])
	clear(r.queue[remaining:])
	r.queue = r.queue[:remaining]
	return batch
}

func (r *Recorder) run() {
	defer close(r.done)
	defer r.sink.Close()
	timer := time.NewTicker(r.limits.FlushInterval)
	defer timer.Stop()
	maintenance := time.NewTicker(time.Minute)
	defer maintenance.Stop()
	batch := make([]record, 0, r.limits.BatchEvents)
	lines := make([][]byte, 0, r.limits.BatchEvents)
	r.maintain(context.Background(), true)
	for {
		select {
		case <-r.stop:
			ctx, cancel := context.WithTimeout(context.Background(), r.limits.Shutdown)
			for ctx.Err() == nil {
				batch = r.take(batch[:0])
				if len(batch) == 0 {
					break
				}
				r.write(ctx, batch, lines[:0])
				clear(batch)
				clear(lines[:cap(lines)])
			}
			r.discard()
			cancel()
			return
		case <-maintenance.C:
			r.maintain(context.Background(), false)
		case <-r.wake:
		case <-timer.C:
		}
		batch = r.take(batch[:0])
		if len(batch) != 0 {
			ctx, cancel := context.WithTimeout(context.Background(), r.limits.Shutdown)
			r.write(ctx, batch, lines[:0])
			cancel()
			clear(batch)
			clear(lines[:cap(lines)])
			r.mu.Lock()
			more := len(r.queue) != 0
			r.mu.Unlock()
			if more {
				select {
				case r.wake <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (r *Recorder) maintain(ctx context.Context, first bool) {
	bounded, cancel := context.WithTimeout(ctx, r.limits.Shutdown)
	defer cancel()
	err := r.sink.Maintain(bounded)
	r.setHealth(err == nil, first)
	if err == nil {
		r.writeLoss(bounded)
	}
}

func (r *Recorder) write(ctx context.Context, batch []record, lines [][]byte) {
	durable := false
	for _, entry := range batch {
		lines = append(lines, entry.data)
		durable = durable || entry.intent != nil
	}
	acknowledge := func(err error) {
		for _, entry := range batch {
			if entry.ack != nil {
				entry.ack <- err
			}
		}
	}
	var err error
	if store, ok := r.sink.(*FileStore); ok {
		var confirm func()
		if durable {
			confirm = func() { acknowledge(nil) }
		}
		err = store.appendRecords(ctx, batch, durable, confirm)
	} else {
		err = r.sink.Append(ctx, lines, durable)
		if err == nil {
			acknowledge(nil)
		}
	}
	if err != nil {
		r.mu.Lock()
		for _, entry := range batch {
			r.addLoss(entry.priority, writeFailure)
		}
		r.mu.Unlock()
		acknowledge(ErrUnavailable)
	}
	r.setHealth(err == nil, false)
	if err == nil {
		r.writeLoss(ctx)
	}
}

func (r *Recorder) setHealth(available, first bool) {
	r.mu.Lock()
	changed := r.healthy != available
	r.healthy = available
	health := Health{Available: available, Lost: r.lost}
	r.mu.Unlock()
	if r.report != nil && (first || changed) {
		r.report(health)
	}
}

func (r *Recorder) writeLoss(ctx context.Context) {
	r.mu.Lock()
	snapshot := r.pending
	r.mu.Unlock()
	if snapshot == (LossCounts{}) {
		return
	}
	at := time.Now().UTC()
	data, err := encode(event{
		Version: 1, ID: eventID(r.namespace, r.sequence.Add(1)), At: at,
		Phase: Loss, Losses: &snapshot,
	}, r.limits.EventBytes)
	if err != nil {
		return
	}
	if store, ok := r.sink.(*FileStore); ok {
		err = store.appendRecords(ctx, []record{{data: data, at: at}}, false, nil)
	} else {
		err = r.sink.Append(ctx, [][]byte{data}, false)
	}
	if err != nil {
		r.setHealth(false, false)
		return
	}
	r.mu.Lock()
	for p := range r.pending {
		for cause, count := range r.pending[p] {
			r.pending[p][cause] = count - snapshot[p][cause]
		}
	}
	r.mu.Unlock()
}

func (r *Recorder) discard() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.queue {
		r.addLoss(entry.priority, shutdown)
		if entry.ack != nil {
			entry.ack <- ErrUnavailable
		}
	}
	clear(r.queue)
	r.queue = r.queue[:0]
	r.bytes = 0
}

func (r *Recorder) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if !r.closing {
		r.closing = true
		close(r.stop)
	}
	r.mu.Unlock()
	r.start.Do(func() {
		_ = r.sink.Close()
		close(r.done)
	})
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
