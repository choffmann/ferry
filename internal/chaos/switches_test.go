package chaos

import (
	"context"
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

func TestLeakerAllocatesAtLeastOneMBWhenSwitchedOn(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()
	l := NewLeaker(st, time.Second)

	mb := 1
	if _, err := st.Apply(ctx, Patch{Resources: &ResourcesPatch{MemoryLeakMBPerMin: &mb}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// One megabyte per minute at a one second tick rounds down to zero, which
	// would make the switch look broken. It is clamped to one instead.
	l.tick(ctx)
	if got := l.AllocatedMB(); got != 1 {
		t.Errorf("AllocatedMB = %d, want 1", got)
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
