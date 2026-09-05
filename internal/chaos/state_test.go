package chaos

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDefaultStateIsHarmless(t *testing.T) {
	s := DefaultState()
	if !Ready(s) {
		t.Error("the default state is not ready")
	}
	if Delay(s) != 0 {
		t.Errorf("Delay = %v, want 0", Delay(s))
	}
	if _, fail := FailWith(s, 0.0); fail {
		t.Error("the default state fails requests")
	}
	if s.Resources.MemoryLeakMBPerMin != 0 {
		t.Errorf("MemoryLeakMBPerMin = %d, want 0", s.Resources.MemoryLeakMBPerMin)
	}
}

func TestApplyOnlyTouchesWhatThePatchNames(t *testing.T) {
	s := DefaultState()
	s.HTTP.LatencyMS = 250
	s.Readiness = "fail"

	var p Patch
	if err := json.Unmarshal([]byte(`{"resources":{"memory_leak_mb_per_min":16}}`), &p); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}

	got := s.Apply(p)
	if got.Resources.MemoryLeakMBPerMin != 16 {
		t.Errorf("MemoryLeakMBPerMin = %d, want 16", got.Resources.MemoryLeakMBPerMin)
	}
	if got.HTTP.LatencyMS != 250 {
		t.Errorf("LatencyMS = %d, the patch did not mention it", got.HTTP.LatencyMS)
	}
	if got.Readiness != "fail" {
		t.Errorf("Readiness = %q, the patch did not mention it", got.Readiness)
	}
}

func TestApplyReachesIntoNestedFields(t *testing.T) {
	s := DefaultState()
	s.HTTP.LatencyMS = 250
	s.HTTP.ErrorRate = 0.5

	var p Patch
	if err := json.Unmarshal([]byte(`{"http":{"latency_ms":10}}`), &p); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}

	got := s.Apply(p)
	if got.HTTP.LatencyMS != 10 {
		t.Errorf("LatencyMS = %d, want 10", got.HTTP.LatencyMS)
	}
	if got.HTTP.ErrorRate != 0.5 {
		t.Errorf("ErrorRate = %v, the patch only named latency_ms", got.HTTP.ErrorRate)
	}
}

func TestMemoryStoreApplyAndReset(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()

	var p Patch
	if err := json.Unmarshal([]byte(`{"readiness":"fail"}`), &p); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	got, err := st.Apply(ctx, p)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if Ready(got) {
		t.Error("still ready after readiness=fail")
	}

	read, err := st.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if Ready(read) {
		t.Error("Get returned a ready state after readiness=fail")
	}

	back, err := st.Reset(ctx)
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if !Ready(back) {
		t.Error("not ready after reset")
	}
}
