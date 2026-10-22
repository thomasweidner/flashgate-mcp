package operation

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestResultStorePutGetCopiesAndDoesNotSlideExpiry(t *testing.T) {
	clock := newTestClock()
	store := newTestStore(t, clock, ResultStoreConfig{MaxEntries: 2, MaxTotalBytes: 8, MaxResultBytes: 4, MaxTTL: time.Hour})
	binding := testBinding()
	input := []byte("data")
	if err := store.Put("op_one", binding, input, time.Minute); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	input[0] = 'X'

	got, err := store.Get("op_one", binding)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got.Data) != "data" || !got.CreatedAt.Equal(clock.start) || !got.ExpiresAt.Equal(clock.start.Add(time.Minute)) {
		t.Fatalf("Get() = %#v", got)
	}
	got.Data[0] = 'X'
	clock.advance(30 * time.Second)
	again, err := store.Get("op_one", binding)
	if err != nil || string(again.Data) != "data" || !again.ExpiresAt.Equal(got.ExpiresAt) {
		t.Fatalf("second Get() = %#v, %v", again, err)
	}
}

func TestResultStoreOwnerChecksAndExpiryAreIndistinguishable(t *testing.T) {
	clock := newTestClock()
	store := newTestStore(t, clock, ResultStoreConfig{MaxEntries: 2, MaxTotalBytes: 8, MaxResultBytes: 8, MaxTTL: time.Hour})
	binding := testBinding()
	if err := store.Put("op_one", binding, []byte("secret"), time.Minute); err != nil {
		t.Fatal(err)
	}
	other := binding
	other.Principal = "other"
	for name, candidate := range map[string]struct {
		handle  string
		binding ResultBinding
	}{
		"missing":     {"op_missing", binding},
		"wrong owner": {"op_one", other},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.Get(candidate.handle, candidate.binding); !errors.Is(err, ErrResultUnavailable) {
				t.Fatalf("Get() error = %v", err)
			}
		})
	}
	clock.advance(time.Minute)
	if _, err := store.Get("op_one", binding); !errors.Is(err, ErrResultUnavailable) {
		t.Fatalf("expired Get() error = %v", err)
	}
	if count, bytes := store.SweepExpired(); count != 0 || bytes != 0 {
		t.Fatalf("SweepExpired() after lazy cleanup = %d, %d", count, bytes)
	}
}

func TestResultStoreEnforcesLimitsAtomicallyAndReclaimsCapacity(t *testing.T) {
	clock := newTestClock()
	store := newTestStore(t, clock, ResultStoreConfig{MaxEntries: 2, MaxTotalBytes: 6, MaxResultBytes: 4, MaxTTL: time.Minute})
	binding := testBinding()
	if err := store.Put("op_one", binding, []byte("1234"), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("op_two", binding, []byte("123"), time.Second); !errors.Is(err, ErrResultLimit) {
		t.Fatalf("total-byte Put() error = %v", err)
	}
	if err := store.Put("op_two", binding, []byte("12345"), time.Second); !errors.Is(err, ErrResultLimit) {
		t.Fatalf("per-result Put() error = %v", err)
	}
	if err := store.Put("op_two", binding, []byte("1"), time.Minute+time.Nanosecond); !errors.Is(err, ErrResultLimit) {
		t.Fatalf("TTL Put() error = %v", err)
	}
	clock.advance(time.Second)
	if count, bytes := store.SweepExpired(); count != 1 || bytes != 4 {
		t.Fatalf("SweepExpired() = %d, %d", count, bytes)
	}
	if err := store.Put("op_two", binding, []byte("1234"), time.Minute); err != nil {
		t.Fatalf("Put() after sweep error = %v", err)
	}
	if err := store.Delete("op_two", binding); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get("op_two", binding); !errors.Is(err, ErrResultUnavailable) {
		t.Fatalf("Get() after Delete error = %v", err)
	}
}

func TestResultStoreConcurrentGetAndDelete(t *testing.T) {
	clock := newTestClock()
	store := newTestStore(t, clock, ResultStoreConfig{MaxEntries: 1, MaxTotalBytes: 4, MaxResultBytes: 4, MaxTTL: time.Minute})
	binding := testBinding()
	if err := store.Put("op_one", binding, []byte("data"), time.Minute); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Get("op_one", binding)
			if err != nil && !errors.Is(err, ErrResultUnavailable) {
				t.Errorf("Get() error = %v", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := store.Delete("op_one", binding); err != nil && !errors.Is(err, ErrResultUnavailable) {
			t.Errorf("Delete() error = %v", err)
		}
	}()
	wg.Wait()
}

func TestNewResultStoreRejectsInvalidConfiguration(t *testing.T) {
	for _, config := range []ResultStoreConfig{
		{},
		{MaxEntries: 1, MaxTotalBytes: 1, MaxResultBytes: 2, MaxTTL: time.Second},
	} {
		if _, err := NewResultStore(config); err == nil {
			t.Fatalf("NewResultStore(%+v) succeeded", config)
		}
	}
}

type testClock struct {
	mu    sync.Mutex
	start time.Time
}

func newTestClock() *testClock {
	return &testClock{start: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)}
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.start
}

func (c *testClock) advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.start = c.start.Add(delta)
}

func newTestStore(t *testing.T, clock *testClock, config ResultStoreConfig) *ResultStore {
	t.Helper()
	store, err := newResultStore(config, clock.now)
	if err != nil {
		t.Fatalf("newResultStore() error = %v", err)
	}
	return store
}

func testBinding() ResultBinding {
	return ResultBinding{
		Principal: "principal", Profile: "profile", Root: "root", ExecutionBackend: "direct", ServiceGeneration: "generation", Domain: "filesystem",
	}
}
