package httpmw

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/choffmann/ferry/internal/obs"
)

func TestRecovererTurnsAPanicIntoFiveHundred(t *testing.T) {
	var buf bytes.Buffer
	h := Recoverer(obs.NewLogger(&buf))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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
	h := Recoverer(obs.NewLogger(&buf))(RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
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
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
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
	h := RequestID(Logging(obs.NewLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestGeneratedRequestIDIsAUUID(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/connections", nil))

	if err := uuid.Validate(seen); err != nil {
		t.Errorf("request id %q is not a uuid: %v", seen, err)
	}
}
