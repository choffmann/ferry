package chaos

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDelayReadsTheLatencySwitch(t *testing.T) {
	s := DefaultState()
	s.HTTP.LatencyMS = 150
	if got := Delay(s); got != 150*time.Millisecond {
		t.Errorf("Delay = %v, want 150ms", got)
	}
}

func TestFailWithUsesTheDrawAndTheConfiguredStatus(t *testing.T) {
	s := DefaultState()
	s.HTTP.ErrorRate = 0.3
	s.HTTP.Status = 503

	if status, fail := FailWith(s, 0.1); !fail || status != 503 {
		t.Errorf("draw below the rate: status %d, fail %v", status, fail)
	}
	if _, fail := FailWith(s, 0.9); fail {
		t.Error("draw above the rate still failed")
	}
}

func TestFailWithFallsBackToFiveHundred(t *testing.T) {
	s := DefaultState()
	s.HTTP.ErrorRate = 1.0
	s.HTTP.Status = 0
	if status, fail := FailWith(s, 0.0); !fail || status != 500 {
		t.Errorf("status = %d, fail = %v, want 500 and true", status, fail)
	}
}

func TestOnlyFailMakesItUnready(t *testing.T) {
	for value, want := range map[string]bool{"ok": true, "fail": false, "": true} {
		s := DefaultState()
		s.Readiness = value
		if got := Ready(s); got != want {
			t.Errorf("Ready with readiness=%q = %v, want %v", value, got, want)
		}
	}
}

// The ticks are driven by hand rather than by Run. A real ticker with a short
// interval would allocate a megabyte per tick until the assertion happens to see
// it, which turns a unit test into a memory bomb.
func TestLeakerAllocatesAndReleases(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	l := NewLeaker(st, 6*time.Second)

	mb := 120
	if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	l.tick(ctx)
	// 120 MB per minute at a six second tick is twelve megabytes.
	if got := l.AllocatedMB(); got != 12 {
		t.Fatalf("AllocatedMB = %d, want 12", got)
	}

	l.tick(ctx)
	if got := l.AllocatedMB(); got != 24 {
		t.Fatalf("AllocatedMB = %d after the second tick, want 24", got)
	}

	if _, err := st.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	l.tick(ctx)
	if got := l.AllocatedMB(); got != 0 {
		t.Errorf("still holding %d MB after the reset", got)
	}
}

// A rate that is not a whole number of megabytes per tick has to carry its
// remainder, or the truncation silently changes the rate: 32 MB per minute at a
// six second tick would allocate three megabytes ten times and deliver 30.
func TestLeakerRateIsExactOverAMinute(t *testing.T) {
	ctx := context.Background()
	for _, mb := range []int{1, 5, 32, 500} {
		st := NewMemoryStore()
		l := NewLeaker(st, 6*time.Second)
		if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		for i := 0; i < 10; i++ {
			l.tick(ctx)
		}
		if got := l.AllocatedMB(); got != mb {
			t.Errorf("%d MB per minute: allocated %d MB after a minute of ticks", mb, got)
		}
	}
}

// Below one megabyte per tick nothing is allocated until the remainder adds up.
// Rounding up instead would turn every rate under ten into ten.
func TestLeakerCarriesARateBelowOneBlockPerTick(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	l := NewLeaker(st, time.Second)

	mb := 1
	if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for i := 0; i < 59; i++ {
		l.tick(ctx)
	}
	if got := l.AllocatedMB(); got != 0 {
		t.Errorf("AllocatedMB = %d after 59 of the 60 ticks that make a megabyte", got)
	}
	l.tick(ctx)
	if got := l.AllocatedMB(); got != 1 {
		t.Errorf("AllocatedMB = %d after a full minute, want 1", got)
	}
}

// A reset must drop the carried remainder too, or the next run starts in debt.
func TestLeakerResetClearsTheCarriedRemainder(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	l := NewLeaker(st, 6*time.Second)

	mb := 5
	if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	l.tick(ctx)

	if _, err := st.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	l.tick(ctx)

	if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i := 0; i < 10; i++ {
		l.tick(ctx)
	}
	if got := l.AllocatedMB(); got != mb {
		t.Errorf("allocated %d MB in the minute after a reset, want %d", got, mb)
	}
}

// Go serves a large allocation from freshly mapped zero pages and skips zeroing
// them, so a block counts against the heap but stays out of RSS until each of its
// pages is written. Touching only the first byte made the switch deliver 2.4% of
// what it promised, which is the whole point of the switch.
func TestLeakerTouchesEveryPageOfEachBlock(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	l := NewLeaker(st, 6*time.Second)

	mb := 10
	if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	l.tick(ctx)

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.blocks) == 0 {
		t.Fatal("the tick allocated nothing")
	}
	page := os.Getpagesize()
	for i, b := range l.blocks {
		for off := 0; off < len(b); off += page {
			if b[off] == 0 {
				t.Fatalf("block %d: the page at offset %d was never written", i, off)
			}
		}
	}
}

func TestLeakerRunStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := NewLeaker(NewMemoryStore(), 10*time.Millisecond)

	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}
