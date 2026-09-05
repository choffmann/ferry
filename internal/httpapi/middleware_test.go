package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/obs"
)

func TestRecovererTurnsAPanicIntoFiveHundred(t *testing.T) {
	var buf bytes.Buffer
	h := recoverer(obs.NewLogger(&buf))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaputt")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/connections", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !bytes.Contains(buf.Bytes(), []byte("kaputt")) {
		t.Errorf("the panic was not logged: %s", buf.String())
	}
}

func TestRecovererComposedWithRequestIDIncludesTheIDInErrorResponse(t *testing.T) {
	var buf bytes.Buffer
	h := recoverer(obs.NewLogger(&buf))(requestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaputt")
	})))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/connections", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}

	headerID := rec.Header().Get("X-Request-Id")
	if headerID == "" {
		t.Fatal("no X-Request-Id header in response")
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["request_id"] != headerID {
		t.Errorf("body request_id = %q, header = %q", body["request_id"], headerID)
	}
}

func TestRequestIDIsGeneratedAndEchoed(t *testing.T) {
	var seen string
	h := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFrom(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/connections", nil))

	if seen == "" {
		t.Fatal("no request id in the context")
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Errorf("header = %q, context = %q", got, seen)
	}
}

func TestRequestIDKeepsTheIncomingOne(t *testing.T) {
	var seen string
	h := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFrom(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/connections", nil)
	req.Header.Set("X-Request-Id", "von-aussen")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "von-aussen" {
		t.Errorf("request id = %q, want von-aussen", seen)
	}
}

func TestLoggingWritesOneLinePerRequest(t *testing.T) {
	var buf bytes.Buffer
	h := requestID(logging(obs.NewLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/connections", nil))

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
	}
	for _, key := range []string{"method", "path", "status", "duration_ms", "request_id"} {
		if _, ok := line[key]; !ok {
			t.Errorf("the log line has no %q: %v", key, line)
		}
	}
	if line["status"] != float64(http.StatusTeapot) {
		t.Errorf("status = %v, want 418", line["status"])
	}
}

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

	// Test error-rate exemption
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

	// Test latency exemption: set a large latency and verify exempt paths return promptly
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

	// Verify that non-exempt paths do get delayed
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
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bookings/nope", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey{}, "abc123"))

	writeError(rec, req, http.StatusNotFound, "Buchung nicht gefunden")

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["error"] != "Buchung nicht gefunden" {
		t.Errorf("error = %q", body["error"])
	}
	if body["request_id"] != "abc123" {
		t.Errorf("request_id = %q, want abc123", body["request_id"])
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
}
