package operation

import (
	"errors"
	"sync"
	"testing"
)

func TestNewConcurrencyLimiterValidatesLimits(t *testing.T) {
	for _, limits := range []ConcurrencyLimits{
		{}, {Global: 1, PerDomain: 0, PerPrincipal: 1},
		{Global: 1, PerDomain: 2, PerPrincipal: 1},
		{Global: 1, PerDomain: 1, PerPrincipal: 2},
	} {
		if _, err := NewConcurrencyLimiter(limits); !errors.Is(err, ErrInvalidConcurrencyLimits) {
			t.Fatalf("NewConcurrencyLimiter(%+v) error = %v", limits, err)
		}
	}
}

func TestConcurrencyLimiterEnforcesEveryScope(t *testing.T) {
	tests := []struct {
		name   string
		limits ConcurrencyLimits
		keys   [][2]string
		want   error
	}{
		{"global", ConcurrencyLimits{2, 2, 2}, [][2]string{{"fs", "a"}, {"search", "b"}, {"process", "c"}}, ErrGlobalConcurrencyLimit},
		{"domain", ConcurrencyLimits{3, 1, 3}, [][2]string{{"fs", "a"}, {"fs", "b"}}, ErrDomainConcurrencyLimit},
		{"principal", ConcurrencyLimits{3, 3, 1}, [][2]string{{"fs", "a"}, {"search", "a"}}, ErrPrincipalConcurrencyLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter, err := NewConcurrencyLimiter(test.limits)
			if err != nil {
				t.Fatal(err)
			}
			for index, key := range test.keys {
				permit, acquireErr := limiter.TryAcquire(key[0], key[1])
				if index == len(test.keys)-1 {
					if permit != nil || !errors.Is(acquireErr, test.want) {
						t.Fatalf("permit=%v error=%v", permit, acquireErr)
					}
					snapshot := limiter.Snapshot()
					if snapshot.Global != len(test.keys)-1 {
						t.Fatalf("rejected admission changed global count: %+v", snapshot)
					}
					continue
				}
				if acquireErr != nil {
					t.Fatal(acquireErr)
				}
				t.Cleanup(permit.Release)
			}
		})
	}
}

func TestConcurrencyLimiterReleaseIsIdempotentAndRemovesKeys(t *testing.T) {
	limiter, _ := NewConcurrencyLimiter(ConcurrencyLimits{1, 1, 1})
	permit, err := limiter.TryAcquire("filesystem", "principal-a")
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() { defer wait.Done(); permit.Release() }()
	}
	wait.Wait()
	snapshot := limiter.Snapshot()
	if snapshot.Global != 0 || len(snapshot.ByDomain) != 0 || len(snapshot.ByPrincipal) != 0 {
		t.Fatalf("unexpected snapshot after release: %+v", snapshot)
	}
	reused, err := limiter.TryAcquire("filesystem", "principal-a")
	if err != nil {
		t.Fatal(err)
	}
	reused.Release()
}

func TestConcurrencyLimiterRejectsNonCanonicalKeysWithoutMutation(t *testing.T) {
	limiter, _ := NewConcurrencyLimiter(DefaultConcurrencyLimits())
	for _, keys := range [][2]string{{"", "a"}, {"fs ", "a"}, {"fs", " a"}} {
		if _, err := limiter.TryAcquire(keys[0], keys[1]); !errors.Is(err, ErrInvalidConcurrencyKey) {
			t.Fatalf("keys=%q error=%v", keys, err)
		}
	}
	if got := limiter.Snapshot().Global; got != 0 {
		t.Fatalf("global=%d", got)
	}
}

func TestConcurrencyLimiterSnapshotIsIndependent(t *testing.T) {
	limiter, _ := NewConcurrencyLimiter(DefaultConcurrencyLimits())
	permit, _ := limiter.TryAcquire("filesystem", "principal-a")
	defer permit.Release()
	snapshot := limiter.Snapshot()
	snapshot.ByDomain["filesystem"] = 99
	if got := limiter.Snapshot().ByDomain["filesystem"]; got != 1 {
		t.Fatalf("domain count=%d", got)
	}
}
