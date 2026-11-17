package resource

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func testLimits() Limits { return Limits{File: 10, Write: 10, Scan: 10, Result: 10, Temp: 10} }

func TestLimitsAndMeters(t *testing.T) {
	l := testLimits()
	if !l.Within(l) {
		t.Fatal("same limit denied")
	}
	if _, err := NewMeter(Limits{}); err == nil {
		t.Fatal("invalid meter")
	}
	for d := File; d < DimensionCount; d++ {
		if l.Check(d, 10) != nil || l.Check(d, 0) != nil || l.Check(d, 11) == nil || l.Check(d, -1) == nil {
			t.Fatal("dimension bound", d)
		}
		m, err := NewMeter(l)
		if err != nil {
			t.Fatal(err)
		}
		if m.Charge(d, 9) != nil || m.Charge(d, 2) == nil || m.Charge(d, 1) != nil || m.Charge(d, 1) == nil || m.Charge(d, -1) == nil {
			t.Fatal("cumulative overflow", d)
		}
	}
	if l.Check(DimensionCount, 0) == nil || l.Cap(DimensionCount) != 0 {
		t.Fatal("unknown dimension")
	}
	l.File = 11
	if l.Within(testLimits()) {
		t.Fatal("expansion allowed")
	}
	l = testLimits()
	l.Temp = 1<<63 - 1
	if l.Validate() == nil {
		t.Fatal("unsafe probe ceiling")
	}
	var m Meter
	if m.Charge(DimensionCount, 0) == nil {
		t.Fatal("unknown charge")
	}
}

func TestConcurrentRootReservations(t *testing.T) {
	first, _ := NewMeter(testLimits())
	second, _ := NewMeter(testLimits())
	release, err := first.ReserveTemp(10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.ReserveTemp(1); err == nil {
		t.Fatal("reservation overflow")
	}
	other, err := second.ReserveTemp(10)
	if err != nil {
		t.Fatal("roots shared a budget")
	}
	defer other()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(release)
	}
	wg.Wait()
	release, err = first.ReserveTemp(10)
	if err != nil {
		t.Fatal("release failed")
	}
	release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	func() {
		release, err := first.ReserveTemp(10)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		if ctx.Err() == nil {
			t.Fatal("fixture")
		}
	}()
	if _, err := first.ReserveTemp(10); err != nil {
		t.Fatal("cancel leaked reservation")
	}
}

// This fixture models an adapter with a verified hard meter at its effect
// boundary. A configured-only backend deliberately has no enforceable meter.
type simulatedBackend struct {
	hard  bool
	meter *Meter
	calls int
}

func (b *simulatedBackend) RequireHardLimits(l Limits, required []Dimension) error {
	if !b.hard {
		return ErrNotEnforceable
	}
	for _, d := range required {
		if d != Write && d != Temp {
			return ErrNotEnforceable
		}
	}
	b.meter, _ = NewMeter(l)
	return nil
}
func (b *simulatedBackend) write(n int64) error {
	if b.meter == nil {
		return ErrNotEnforceable
	}
	if err := b.meter.Charge(Write, n); err != nil {
		return err
	}
	b.calls++
	return nil
}

func TestBackendEnforcementParity(t *testing.T) {
	for _, name := range []string{"simulated metered backend"} {
		t.Run(name, func(t *testing.T) {
			b := &simulatedBackend{hard: true}
			if err := Admit(b, testLimits(), Write, Temp); err != nil {
				t.Fatal(err)
			}
			if b.write(10) != nil || !errors.Is(b.write(1), ErrLimitExceeded) || b.calls != 1 {
				t.Fatal("effect limit bypass")
			}
		})
	}
	for _, b := range []*simulatedBackend{{}, {hard: true}} {
		if Admit(b, testLimits(), Scan) == nil || b.calls != 0 {
			t.Fatal("missing hard enforcement dispatched")
		}
	}
	if Admit(nil, testLimits(), Write) == nil || Admit(&simulatedBackend{}, Limits{}, Write) == nil || Admit(&simulatedBackend{}, testLimits()) == nil || Admit(&simulatedBackend{}, testLimits(), DimensionCount) == nil {
		t.Fatal("invalid admission")
	}
}

func TestResultContextOnlyTightens(t *testing.T) {
	ctx := WithResultLimit(context.Background(), 10)
	ctx = WithResultLimit(ctx, 20)
	if ResultLimit(ctx, 30) != 10 || ResultLimit(ctx, 5) != 5 || ResultLimit(context.Background(), 5) != 5 {
		t.Fatal("response expansion")
	}
}

func TestAdapterResultOverheadOnlyTightens(t *testing.T) {
	ctx := WithResultLimit(context.Background(), 10)
	ctx = WithResultOverhead(ctx, 3)
	if ResultLimit(ctx, 20) != 7 || ResultLimit(ctx, 5) != 2 {
		t.Fatal("overhead not applied to both ceilings")
	}
	ctx = WithResultLimit(ctx, 20)
	if ResultLimit(ctx, 20) != 7 {
		t.Fatal("overhead charged twice")
	}
	ctx = WithResultOverhead(ctx, 2)
	if ResultLimit(ctx, 20) != 5 {
		t.Fatal("overhead reservation not cumulative")
	}
	if ResultLimit(WithResultLimit(ctx, 4), 10) >= 0 {
		t.Fatal("oversize metadata admitted")
	}
	if ResultLimit(WithResultOverhead(ctx, -1), 10) >= 0 {
		t.Fatal("negative overhead widened allowance")
	}
	if ResultLimit(WithResultOverhead(ctx, 1<<63-1), 10) >= 0 {
		t.Fatal("overhead overflow")
	}
}
