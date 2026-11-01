package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/store"
)

func newIndexServer(t *testing.T, ping, paymentsPing func(context.Context) error) http.Handler {
	t.Helper()
	return NewRouter(Deps{
		Repo:         store.NewFake(time.Now().UTC()),
		Chaos:        chaos.NewMemoryStore(),
		AdminToken:   "test-token",
		Logger:       obs.NewLogger(io.Discard),
		Tickets:      testRenderer(t),
		Draw:         func() float64 { return 1 },
		Ping:         ping,
		PaymentsPing: paymentsPing,
	})
}

func up(context.Context) error   { return nil }
func down(context.Context) error { return errors.New("connection refused") }

func serviceRow(name, status string) string {
	return "<tr><td>" + name + "</td><td>" + status + "</td></tr>"
}

func TestIndexShowsBuildInfoAndServiceStatus(t *testing.T) {
	h := newIndexServer(t, up, down)

	rec := do(t, h, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"Version", "Commit", "Build time",
		serviceRow("database", statusReachable),
		serviceRow("payments", statusUnreachable),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("start page lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "connection refused") {
		t.Error("start page leaks the probe error, which belongs in the log")
	}
}

func TestIndexMarksAnUnsetPaymentsURL(t *testing.T) {
	h := newIndexServer(t, up, nil)

	body := do(t, h, http.MethodGet, "/", "").Body.String()
	if !strings.Contains(body, serviceRow("payments", statusNotConfigured)) {
		t.Errorf("payments not marked as not configured:\n%s", body)
	}
}

func TestIndexOnlyAnswersTheRoot(t *testing.T) {
	h := newIndexServer(t, up, up)

	if rec := do(t, h, http.MethodGet, "/nowhere", ""); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nowhere = %d, want 404", rec.Code)
	}
}
