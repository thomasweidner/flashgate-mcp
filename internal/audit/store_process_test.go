//go:build windows || linux

package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const storeWorkerEnv = "FLASHGATE_TEST_AUDIT_WORKER"

func TestAuditStoreProcessWorker(t *testing.T) {
	root := os.Getenv(storeWorkerEnv)
	if root == "" {
		return
	}
	store, err := NewFileStore(root, "shared", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte{'x'}, 180)
	var batch [][]byte
	for sequence := range 12 {
		batch = append(batch, fmt.Appendf(nil, "{\"writer\":%d,\"sequence\":%d,\"data\":\"%s\"}\n", os.Getpid(), sequence, data))
	}
	for range 3 {
		if err := store.Append(t.Context(), batch, false); err != nil {
			t.Fatal(err)
		}
	}
	// The child exits without explicitly releasing either native handle.
	// A later process must still admit the store and the shared byte limit.
}

func TestStoreParentSyncGatesDurableAppend(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "audit")
	limits := StoreLimits{12 << 10, 4 << 10, 24 * time.Hour}
	lines := [][]byte{[]byte("{\"value\":1}\n")}
	syncFailure := errors.New("fixture parent sync failure")
	for _, name := range []string{"new_store", "peer_observes_existing_store"} {
		t.Run(name, func(t *testing.T) {
			store, err := NewFileStore(path, "fixture", limits)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			store.syncParent = func(root *os.Root) error {
				if !nativeSamePath(root.Name(), filepath.Clean(parent)) {
					t.Fatalf("sync targeted the wrong parent: %s", root.Name())
				}
				return syncFailure
			}
			if err := store.Append(t.Context(), lines, true); !errors.Is(err, syncFailure) {
				t.Fatalf("durable append must propagate the parent sync failure: %v", err)
			}
			if entries, err := os.ReadDir(path); err != nil || len(entries) != 0 {
				t.Fatalf("store should contain no coordination files or records: %v, %v", entries, err)
			}
		})
	}

	store, err := NewFileStore(path, "fixture", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	nativeSync := store.syncParent
	calls := 0
	store.syncParent = func(root *os.Root) error {
		calls++
		return nativeSync(root)
	}
	for range 2 {
		if err := store.Append(t.Context(), lines, true); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("parent sync calls across two appends: got %d, want 1", calls)
	}
}

func TestAuditStoreTenProcesses(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	errs := make(chan error, 10)
	for range 10 {
		workers.Go(func() {
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestAuditStoreProcessWorker$")
			cmd.Env = append(os.Environ(), storeWorkerEnv+"="+root)
			if output, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("audit worker failed: %w: %s", err, output)
			}
		})
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	store, err := NewFileStore(root, "shared", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(ctx); err != nil {
		t.Fatal(err)
	}
	var size int64
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err == nil {
			size += info.Size()
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if size <= 4096 || size > 12<<10 {
		t.Fatalf("shared byte bound: %d", size)
	}
}

func TestStorePolicyLeaseAndTailRecovery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	old, err := NewFileStore(root, "same", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	newer, err := NewFileStore(root, "same", StoreLimits{24 << 10, 4 << 10, 48 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer newer.Close()
	before := []byte("{\"complete\":1}\n")
	if err := old.Append(t.Context(), [][]byte{before}, true); err != nil {
		t.Fatal(err)
	}
	if err := newer.Maintain(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("concurrent store policy update: %v", err)
	}
	name := filepath.Join(root, old.name)
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{\"interrupted\":"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	after := []byte("{\"complete\":2}\n")
	if err := newer.Append(t.Context(), [][]byte{after}, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(name)
	if err != nil || !bytes.HasSuffix(got, append(bytes.Clone(before), after...)) {
		t.Fatalf("append boundary not recovered: %q, %v", got, err)
	}
}

func TestRetentionAcrossDayAndRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{12 << 10, 4 << 10, 24 * time.Hour}
	store, err := NewFileStore(root, "retention", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	clock := time.Date(2026, 11, 20, 23, 59, 0, 0, time.UTC)
	store.now = func() time.Time { return clock }
	record := [][]byte{[]byte("{\"outcome\":\"completed\"}\n")}
	if err := store.Append(t.Context(), record, false); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, store.name)
	clock = clock.Add(time.Minute)
	if err := store.Append(t.Context(), record, false); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, store.name)
	if first == second {
		t.Fatal("UTC transition did not rotate")
	}
	clock = clock.Add(-time.Hour)
	if err := store.Append(t.Context(), record, false); err != nil || filepath.Join(root, store.name) != second {
		t.Fatalf("backward UTC step reopened older partition: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	resumed, err := NewFileStore(root, "retention", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	clock = time.Date(2026, 11, 21, 23, 59, 0, 0, time.UTC)
	resumed.now = func() time.Time { return clock }
	if err := resumed.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("expired partition visible after startup maintenance")
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if err := resumed.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(second); !os.IsNotExist(err) {
		t.Fatal("the second partition survived its UTC day-start deadline")
	}
}

func TestRetentionUsesObservationDayForDelayedRecordsAndRollback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{12 << 10, 4 << 10, 48 * time.Hour}
	store, err := NewFileStore(root, "observation-days", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	day := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	clock := day.Add(24*time.Hour + time.Minute)
	store.now = func() time.Time { return clock }
	current := record{data: []byte("{\"current\":true}\n"), at: clock}
	delayed := record{data: []byte("{\"delayed\":true}\n"), at: day.Add(time.Minute)}
	if err := store.appendRecords(t.Context(), []record{current, delayed}, false, nil); err != nil {
		t.Fatal(err)
	}
	if len(store.entries) != 2 || !store.entries[0].day.Equal(day) {
		t.Fatalf("delayed record used the persistence day: %+v", store.entries)
	}
	old := filepath.Join(root, store.entries[0].name)
	clock = day.Add(48 * time.Hour)
	if err := store.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("expired delayed record survived: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(-time.Hour)
	resumed, err := NewFileStore(root, "observation-days", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	resumed.now = func() time.Time { return clock }
	if err := resumed.appendRecords(t.Context(), []record{delayed}, true, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("clock rollback after restart admitted an expired record: %v", err)
	}
	if len(resumed.entries) != 1 || !resumed.entries[0].day.Equal(day.Add(24*time.Hour)) {
		t.Fatalf("unexpected partition inventory: %+v", resumed.entries)
	}
}

func TestLatePartialAppendReconcilesSharedByteBudget(t *testing.T) {
	for _, nextWriter := range []string{"same_writer", "peer"} {
		t.Run(nextWriter, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "audit")
			limits := StoreLimits{8 << 10, 4 << 10, 7 * 24 * time.Hour}
			writer, err := NewFileStore(root, "partial-append", limits)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			peer, err := NewFileStore(root, "partial-append", limits)
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			now := time.Date(2026, time.November, 25, 12, 0, 0, 0, time.UTC)
			writer.now = func() time.Time { return now }
			peer.now = writer.now
			ctx := t.Context()
			line := func(b byte) []byte {
				return fmt.Appendf(nil, "{\"data\":\"%s\"}\n", bytes.Repeat([]byte{b}, 3000))
			}
			seed, after := line('a'), line('c')
			if err := writer.Append(ctx, [][]byte{seed}, true); err != nil {
				t.Fatal(err)
			}
			seedPath := filepath.Join(root, writer.name)
			if err := peer.Maintain(ctx); err != nil {
				t.Fatal(err)
			}
			late := record{data: line('b'), at: now.Add(-24 * time.Hour)}
			var latePath string
			err = writer.withLock(ctx, func() error {
				if err := writer.rotate(now, recordDay(late, now), int64(len(late.data)), nil); err != nil {
					return err
				}
				latePath = filepath.Join(root, writer.name)
				if _, err := writer.prune(now, int64(len(late.data)), true, nil); err != nil {
					return err
				}
				// Materialize a short write on the native handle and deliver
				// its error to the same finalization used by appendLines.
				if _, err := writer.active.WriteAt(late.data[:2400], int64(len(writer.header))); err != nil {
					return err
				}
				return writer.finishAppend(io.ErrShortWrite, true)
			})
			if !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("expected partial append failure: %v", err)
			}
			if info, err := os.Stat(latePath); err != nil || info.Size() != int64(len(writer.header))+2400 {
				t.Fatalf("older segment did not contain the partial write: %v, %v", info, err)
			}
			physicalSize := func() int64 {
				t.Helper()
				var size int64
				err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					info, err := entry.Info()
					if err == nil {
						size += info.Size()
					}
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				return size
			}
			before := physicalSize()
			rotation := int64(len(writer.header) + len(after))
			if before > limits.Bytes || before+rotation <= limits.Bytes || before-2400+rotation > limits.Bytes {
				t.Fatalf("fixture does not distinguish stale and actual admission: %d + %d", before, rotation)
			}
			next := writer
			if nextWriter == "peer" {
				marker := []byte{0}
				if _, err := peer.lock.ReadAt(marker, int64(len(writer.binding))); err != nil || marker[0] != 'D' {
					t.Fatalf("partial write did not invalidate the shared inventory: %q, %v", marker, err)
				}
				next = peer
			}
			if err := next.Append(ctx, [][]byte{after}, true); err != nil {
				t.Fatal(err)
			}
			if size := physicalSize(); size > limits.Bytes {
				t.Fatalf("partial append escaped the shared byte budget: %d > %d", size, limits.Bytes)
			}
			if _, err := os.Stat(latePath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected the oldest segment to be pruned: %v", err)
			}
			for path, want := range map[string][]byte{seedPath: seed, filepath.Join(root, next.name): after} {
				if data, err := os.ReadFile(path); err != nil || !bytes.HasSuffix(data, want) {
					t.Fatalf("reconciliation did not retain a completed append: %v", err)
				}
			}
		})
	}
}

func TestDurableBatchRefusesConfirmationWhenBudgetCannotRetainIt(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	store, err := NewFileStore(root, "inflight-budget", StoreLimits{8 << 10, 4 << 10, 48 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := NewRecorder(recorderLimits(), "inflight-budget", [32]byte{1}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer recorder.Close(t.Context())
	line := []byte(fmt.Sprintf("{\"value\":%q}\n", string(bytes.Repeat([]byte("a"), 3<<10-64))))
	first, last := make(chan error, 1), make(chan error, 1)
	records := []record{
		{data: line, intent: t.Context(), ack: first},
		{data: line},
		{data: line, intent: t.Context(), ack: last},
	}
	recorder.write(t.Context(), records, nil)
	for _, ack := range []chan error{first, last} {
		select {
		case err := <-ack:
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("in-flight intent was confirmed outside available budget: %v", err)
			}
		default:
			t.Fatal("intent did not resolve")
		}
	}
	if len(store.entries) != 2 {
		t.Fatalf("earlier in-flight segments were pruned: %+v", store.entries)
	}
	var bytesOnDisk int64
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err == nil {
			bytesOnDisk += info.Size()
		}
		return err
	}); err != nil || bytesOnDisk > 8<<10 {
		t.Fatalf("store exceeded the physical-file budget: %d (%v)", bytesOnDisk, err)
	}
}

func TestDurableConfirmationPrecedesCompetingWriter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{8 << 10, 4 << 10, 48 * time.Hour}
	first, err := NewFileStore(root, "confirmation-lock", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	peer, err := NewFileStore(root, "confirmation-lock", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	intentRecord := record{data: []byte("{\"intent\":true}\n")}
	var called bool
	err = first.appendRecords(t.Context(), []record{intentRecord}, true, func() {
		called = true
		locked, err := lockFile(peer.lock, true)
		if locked {
			unlockFile(peer.lock)
		}
		if err != nil || locked {
			t.Fatalf("a competing writer acquired the lock before confirmation: %v / %v", locked, err)
		}
		file, err := os.ReadFile(filepath.Join(root, first.name))
		if err != nil || !bytes.HasSuffix(file, intentRecord.data) {
			t.Fatalf("confirmed intent was absent under the lock: %v", err)
		}
	})
	if err != nil || !called {
		t.Fatalf("eligible intent was not confirmed: %v / %v", called, err)
	}
	if err := peer.Append(t.Context(), [][]byte{[]byte("{\"peer\":true}\n")}, true); err != nil {
		t.Fatal(err)
	}
}

func TestStorePreservesUnknownFilesAndRefusesLinkedNodes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{12 << 10, 4 << 10, 24 * time.Hour}
	store, err := NewFileStore(root, "owner", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Append(t.Context(), [][]byte{[]byte("{\"before\":1}\n")}, false); err != nil {
		t.Fatal(err)
	}
	segment := filepath.Join(root, store.name)
	alias := filepath.Join(filepath.Dir(root), "alias.jsonl")
	if err := os.Link(segment, alias); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(t.Context(), [][]byte{[]byte("{\"after\":2}\n")}, false); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsafe linked segment was accepted: %v", err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(segment)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("link failure changed the segment")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(root, "foreign.txt")
	const contents = "unmanaged fixture"
	if err := os.WriteFile(unknown, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	resumed, err := NewFileStore(root, "owner", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	if err := resumed.Maintain(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unproved namespace was accepted: %v", err)
	}
	data, err := os.ReadFile(unknown)
	if err != nil || string(data) != contents {
		t.Fatal("unknown data was changed")
	}
}

func TestStoreBoundedLockWaitAndCorruptTailRotation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{12 << 10, 4 << 10, 24 * time.Hour}
	if _, err := NewFileStore("relative", "fixture", limits); !errors.Is(err, ErrLimits) {
		t.Fatalf("expected unsafe store path rejection: %v", err)
	}
	store, err := NewFileStore(root, "fixture", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Append(t.Context(), [][]byte{[]byte("{\"valid\":true}\n")}, false); err != nil {
		t.Fatal(err)
	}
	second, err := NewFileStore(root, "fixture", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if ok, err := lockFile(store.lock, true); !ok || err != nil {
		t.Fatalf("fixture lock could not be acquired: %v", err)
	}
	deadline, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()
	err = second.Maintain(deadline)
	unlockFile(store.lock)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contested lock did not respect deadline: %v", err)
	}
	if err := second.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := second.Append(t.Context(), [][]byte{make([]byte, limits.SegmentBytes)}, false); !errors.Is(err, ErrLimits) {
		t.Fatalf("oversized record entered the store: %v", err)
	}
	firstName := store.name
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(root, firstName), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("broken JSON record\n")
	if _, err := file.Write(corrupt); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Append(t.Context(), [][]byte{[]byte("{\"recovered\":true}\n")}, false); err != nil {
		t.Fatal(err)
	}
	if second.name == firstName {
		t.Fatal("an invalid closed record was reused as a writable segment")
	}
	old, err := os.ReadFile(filepath.Join(root, firstName))
	if err != nil || !bytes.HasSuffix(old, corrupt) {
		t.Fatal("rotation rewrote previous unknown data")
	}
}

func TestStoreRecoversBeyondInitialTailRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	store, err := NewFileStore(root, "fixture", StoreLimits{128 << 10, 32 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	line := append([]byte(`{"entry":"`), bytes.Repeat([]byte{'x'}, 8<<10)...)
	line = append(line, '"', '}', '\n')
	if err := store.Append(t.Context(), [][]byte{line}, true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, store.name)
	incomplete := append([]byte(`{"partial":"`), bytes.Repeat([]byte{'x'}, 5<<10)...)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(incomplete)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	final := []byte("{\"next\":true}\n")
	if err := store.Append(t.Context(), [][]byte{final}, true); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	want := bytes.Join([][]byte{store.header, line, final}, nil)
	if err != nil || !bytes.Equal(content, want) {
		t.Fatalf("repair did not preserve the confirmed records: %v", err)
	}
}

func TestStoreRejectsUnprovedCoordinationFiles(t *testing.T) {
	for _, fault := range []string{"lease_size", "lock_size", "store_object"} {
		t.Run(fault, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "audit")
			store, err := NewFileStore(root, "fixture", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if fault == "store_object" {
				if err := os.WriteFile(root, []byte("foreign fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := store.Maintain(t.Context()); err != nil {
					t.Fatal(err)
				}
				var file *os.File
				var data []byte
				if fault == "lease_size" {
					file, data = store.lease, []byte("unexpected")
				} else {
					file, data = store.lock, bytes.Repeat([]byte{'x'}, 512)
				}
				if _, err := file.WriteAt(data, 0); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Maintain(t.Context()); err == nil {
				t.Fatal("unproved store coordination was accepted")
			}
		})
	}
}

func TestStoreReconcilesInterruptedEpochAndSmallerSegmentPolicy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	original, err := NewFileStore(root, "stable", StoreLimits{32 << 10, 8 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	record := append([]byte("{\"data\":\""), bytes.Repeat([]byte{'x'}, 5000)...)
	record = append(record, []byte("\"}\n")...)
	if err := original.Append(t.Context(), [][]byte{record}, false); err != nil {
		t.Fatal(err)
	}
	oldName := original.name
	if err := original.withLock(t.Context(), original.invalidate); err != nil {
		t.Fatal(err)
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := NewFileStore(root, "stable", StoreLimits{32 << 10, 8 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if err := recovered.Maintain(t.Context()); err != nil || len(recovered.epoch) != epochSize || recovered.epoch[0] != 'C' {
		t.Fatalf("interrupted metadata was not rebound: %q, %v", recovered.epoch, err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	smaller, err := NewFileStore(root, "stable", StoreLimits{24 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer smaller.Close()
	if err := smaller.Append(t.Context(), [][]byte{[]byte("{\"resumed\":true}\n")}, true); err != nil {
		t.Fatal(err)
	}
	if smaller.name == oldName {
		t.Fatal("former policy's oversized segment was reopened")
	}
	old, err := os.ReadFile(filepath.Join(root, oldName))
	if err != nil || !bytes.HasSuffix(old, record) {
		t.Fatal("older completed segment was changed")
	}
}

func TestLiveStoreDoesNotRebindReplacedCoordinator(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	store, err := NewFileStore(root, "stable", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(t.Context()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, lockName)
	if err := os.Rename(path, filepath.Join(root, "held.lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("foreign coordinator"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Maintain(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("replaced coordinator accepted: %v", err)
	}
}

func TestInterruptedRotationRebindsPeerWithoutChangingClosedSegment(t *testing.T) {
	root := filepath.Join(t.TempDir(), "audit")
	limits := StoreLimits{32 << 10, 4 << 10, 7 * 24 * time.Hour}
	writer, err := NewFileStore(root, "fixture", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	peer, err := NewFileStore(root, "fixture", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	clock := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	writer.now = func() time.Time { return clock }
	peer.now = writer.now
	ctx := t.Context()
	first := []byte("{\"phase\":\"first\"}\n")
	if err := peer.Append(ctx, [][]byte{first}, true); err != nil {
		t.Fatal(err)
	}
	prior := filepath.Join(root, peer.name)
	stale := peer.active
	priorBytes, err := os.ReadFile(prior)
	if err != nil {
		t.Fatal(err)
	}

	clock = clock.Add(24 * time.Hour)
	var blocked string
	if err := writer.withLock(ctx, func() error {
		dir, err := writer.putDay(clock.Truncate(24 * time.Hour))
		blocked = filepath.Join(root, dir, "audit-000001.jsonl")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("unmanaged rotation collision\n")
	if err := os.WriteFile(blocked, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	retrying := []byte("{\"phase\":\"rotation\"}\n")
	if err := writer.Append(ctx, [][]byte{retrying}, true); !errors.Is(err, os.ErrExist) {
		t.Fatalf("rotation did not fail on the occupied segment name: %v", err)
	}
	if content, err := os.ReadFile(blocked); err != nil || !bytes.Equal(content, foreign) {
		t.Fatalf("failed rotation changed the unrelated object: %v", err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := writer.Append(ctx, [][]byte{retrying}, true); err != nil {
		t.Fatal(err)
	}
	resumed := []byte("{\"phase\":\"peer-resumed\"}\n")
	if err := peer.Append(ctx, [][]byte{resumed}, true); err != nil {
		t.Fatal(err)
	}
	if peer.active == stale {
		t.Fatal("peer reused its pre-rotation handle")
	}
	if _, err := stale.ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("peer left the pre-rotation handle readable")
	}
	if content, err := os.ReadFile(prior); err != nil || !bytes.Equal(content, priorBytes) {
		t.Fatalf("closed segment changed after peer resumed: %v", err)
	}
	if content, err := os.ReadFile(blocked); err != nil || !bytes.HasSuffix(content, bytes.Join([][]byte{retrying, resumed}, nil)) {
		t.Fatalf("new segment does not contain the confirmed records in order: %v", err)
	}
}

func TestStoreFailsClosedOnMissingParentAndNonFileCoordinators(t *testing.T) {
	for _, fault := range []string{"parent_absent", lockName, leaseName} {
		t.Run(fault, func(t *testing.T) {
			parent := t.TempDir()
			if fault == "parent_absent" {
				parent = filepath.Join(parent, "missing")
			}
			root := filepath.Join(parent, "audit")
			store, err := NewFileStore(root, "fixture", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if fault != "parent_absent" {
				if err := makeOwnedStoreFixture(parent); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(root, fault), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Maintain(t.Context()); err == nil {
				t.Fatal("unsafe or unavailable coordination was admitted")
			}
			if fault != "parent_absent" {
				if info, err := os.Stat(filepath.Join(root, fault)); err != nil || !info.IsDir() {
					t.Fatal("foreign coordinator was deleted or replaced")
				}
			}
		})
	}
}

func makeOwnedStoreFixture(parent string) error {
	directory, err := os.OpenRoot(filepath.Clean(parent))
	if err != nil {
		return err
	}
	defer directory.Close()
	return makeStoreDirectory(directory, "audit")
}
