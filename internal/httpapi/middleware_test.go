package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/httpmw"
)

func TestChaosMiddlewareDelaysAndFails(t *testing.T) {
	st := chaos.NewMemoryStore()
	ctx := context.Background()

	latency := 40
	rate := 1.0
	if _, err := st.Apply(ctx, chaos.Patch{HTTP: &chaos.HTTPPatch{LatencyMS: &latency, ErrorRate: &rate}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	h := chaosMiddleware(st, func() float64 { return 0 })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the handler ran although the error rate was 1.0")
	}))

	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/connections", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("the request took %v, the latency switch says at least 40ms", elapsed)
	}
}

func TestChaosMiddlewareSparesAdminAndHealthz(t *testing.T) {
	st := chaos.NewMemoryStore()
	ctx := context.Background()

	rate := 1.0
	if _, err := st.Apply(ctx, chaos.Patch{HTTP: &chaos.HTTPPatch{ErrorRate: &rate}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, path := range []string{"/admin/chaos", "/healthz"} {
		reached := false
		h := chaosMiddleware(st, func() float64 { return 0 })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		if !reached {
			t.Errorf("%s was hit by the chaos middleware error-rate, which would lock the operator out", path)
		}
	}

	latencyMS := 100
	zeroRate := 0.0
	if _, err := st.Apply(ctx, chaos.Patch{HTTP: &chaos.HTTPPatch{LatencyMS: &latencyMS, ErrorRate: &zeroRate}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, path := range []string{"/admin/chaos", "/healthz"} {
		reached := false
		start := time.Now()
		h := chaosMiddleware(st, func() float64 { return 0 })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		elapsed := time.Since(start)

		if !reached {
			t.Errorf("%s was hit by the chaos middleware latency, which would lock the operator out", path)
		}
		if elapsed >= 50*time.Millisecond {
			t.Errorf("%s took %v, seems to have been delayed despite exemption", path, elapsed)
		}
	}

	nonExempt := "/bookings"
	reached := false
	start := time.Now()
	h := chaosMiddleware(st, func() float64 { return 0 })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, nonExempt, nil))
	elapsed := time.Since(start)

	if !reached {
		t.Error("non-exempt path should have reached the handler")
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("%s took %v, should have been delayed by at least 100ms", nonExempt, elapsed)
	}
}

func TestWriteErrorCarriesTheRequestID(t *testing.T) {
	h := httpmw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "booking not found")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bookings/nope", nil)
	req.Header.Set("X-Request-Id", "abc123")
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["error"] != "booking not found" {
		t.Errorf("error = %q", body["error"])
	}
	if body["request_id"] != "abc123" {
		t.Errorf("request_id = %q, want abc123", body["request_id"])
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
}
