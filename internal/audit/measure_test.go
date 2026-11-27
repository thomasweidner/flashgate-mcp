//go:build windows || linux

package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type measurementSink struct {
	stall bool
}

func (s measurementSink) Append(ctx context.Context, _ [][]byte, _ bool) error {
	if s.stall {
		<-ctx.Done()
		return ctx.Err()
	}
	return syscall.ENOSPC
}
func (measurementSink) Maintain(context.Context) error { return nil }
func (measurementSink) Close() error                   { return nil }

// These fixtures measure controlled failure and recovery separately from
// productive STDIO and never exhaust a real disk.
func BenchmarkAuditFaults(b *testing.B) {
	for _, tc := range []struct {
		name string
		sink measurementSink
	}{
		{"disk-full", measurementSink{}},
		{"slow-sink", measurementSink{stall: true}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			limits := recorderLimits()
			limits.Shutdown = 100 * time.Millisecond
			limits.IntentTimeout = 10 * time.Millisecond
			r, err := NewRecorder(limits, "measure", [32]byte{4}, tc.sink, nil)
			if err != nil {
				b.Fatal(err)
			}
			r.Start()
			defer func() { _ = r.Close(b.Context()) }()
			observation := Observation{Request: Reference{"measure", 0}, Phase: Intent, Reason: Allowed}
			for b.Loop() {
				observation.Request.Request++
				if _, err := r.Require(b.Context(), observation); !errors.Is(err, ErrUnavailable) {
					b.Fatalf("fault fixture could confirm intent: %v", err)
				}
			}
			health := r.Health()
			for cause, count := range health.Lost[Low] {
				if count != 0 {
					b.ReportMetric(float64(count)/float64(b.N), lossMetric(cause))
				}
			}
		})
	}
}

func BenchmarkAuditQueuePressure(b *testing.B) {
	limits := recorderLimits()
	recorder, err := NewRecorder(limits, "measure", [32]byte{4}, measurementSink{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	// Hold the consumer back to model a saturated queue deterministically.
	observation := Observation{Request: Reference{"measure", 1}, Phase: Decision, Reason: Allowed}
	for range limits.QueueEvents {
		recorder.Record(observation)
	}
	if len(recorder.queue) != limits.QueueEvents {
		b.Fatal("fixture did not reach the configured queue count")
	}
	for b.Loop() {
		recorder.Record(observation)
	}
	b.ReportMetric(float64(recorder.bytes), "queued_B")
	b.ReportMetric(float64(len(recorder.queue)), "queued_events")
	b.ReportMetric(float64(recorder.Health().Lost[Low][pressure])/float64(b.N), "queue-loss/op")
}

func lossMetric(cause int) string {
	return [...]string{"pressure/op", "projection-loss/op", "failed-writes/op", "shutdown-loss/op", "intent-timeouts/op"}[cause]
}

func BenchmarkAuditNativeAppend(b *testing.B) {
	line := []byte("{\"result\":\"completed\"}\n")
	for _, durable := range []bool{false, true} {
		for _, size := range []int{1, 32, 128} {
			b.Run(fmt.Sprintf("durable-%t/batch-%d", durable, size), func(b *testing.B) {
				root := filepath.Join(b.TempDir(), "audit")
				store, err := NewFileStore(root, "measure", StoreLimits{32 << 20, 4 << 20, 24 * time.Hour})
				if err != nil {
					b.Fatal(err)
				}
				defer store.Close()
				if err := store.Append(b.Context(), [][]byte{line}, durable); err != nil {
					b.Fatal(err)
				}
				batch := make([][]byte, size)
				for index := range batch {
					batch[index] = line
				}
				b.SetBytes(int64(size * len(line)))
				for b.Loop() {
					if err := store.Append(b.Context(), batch, durable); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkAuditNativeRecovery(b *testing.B) {
	root := filepath.Join(b.TempDir(), "audit")
	store, err := NewFileStore(root, "measure", StoreLimits{32 << 20, 1 << 20, 24 * time.Hour})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	line := []byte("{\"result\":\"completed\"}\n")
	if err := store.Append(b.Context(), [][]byte{line}, true); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		b.StopTimer()
		activePath := filepath.Join(root, store.name)
		file, err := os.OpenFile(activePath, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			b.Fatal(err)
		}
		_, writeErr := file.WriteString("{\"partial\":")
		if err := errors.Join(writeErr, file.Close()); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if err := store.Append(b.Context(), [][]byte{line}, true); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(line)))
}

func BenchmarkAuditNativeLockWait(b *testing.B) {
	root := filepath.Join(b.TempDir(), "audit")
	limits := StoreLimits{12 << 10, 4 << 10, 24 * time.Hour}
	holder, err := NewFileStore(root, "measure", limits)
	if err != nil {
		b.Fatal(err)
	}
	defer holder.Close()
	peer, err := NewFileStore(root, "measure", limits)
	if err != nil {
		b.Fatal(err)
	}
	defer peer.Close()
	for _, store := range []*FileStore{holder, peer} {
		if err := store.Maintain(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
	for b.Loop() {
		ok, err := lockFile(holder.lock, true)
		if !ok || err != nil {
			b.Fatalf("controlled holder did not acquire lock: %v", err)
		}
		released := make(chan struct{})
		time.AfterFunc(2*time.Millisecond, func() {
			unlockFile(holder.lock)
			close(released)
		})
		err = awaitLock(b.Context(), peer.lock)
		<-released
		if err != nil {
			b.Fatal(err)
		}
		unlockFile(peer.lock)
	}
}

func BenchmarkAuditNativeRetention(b *testing.B) {
	parent := b.TempDir()
	path := filepath.Join(parent, "audit")
	limits := StoreLimits{128 << 10, 16 << 10, 24 * time.Hour}
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	line := []byte("{\"result\":\"completed\"}\n")
	for b.Loop() {
		b.StopTimer()
		previous, err := NewFileStore(path, "measure", limits)
		if err != nil {
			b.Fatal(err)
		}
		previous.now = func() time.Time { return clock }
		appendErr := previous.Append(b.Context(), [][]byte{line}, true)
		oldSegment := previous.name
		if err := errors.Join(appendErr, previous.Close()); err != nil {
			b.Fatal(err)
		}
		clock = clock.Add(48 * time.Hour)
		resumed, err := NewFileStore(path, "measure", limits)
		if err != nil {
			b.Fatal(err)
		}
		resumed.now = func() time.Time { return clock }
		b.StartTimer()
		if err := errors.Join(resumed.Maintain(b.Context()), resumed.Close()); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if _, err := os.Stat(filepath.Join(path, oldSegment)); !errors.Is(err, os.ErrNotExist) {
			b.Fatalf("expired segment after restart: %v", err)
		}
		b.StartTimer()
	}
}
