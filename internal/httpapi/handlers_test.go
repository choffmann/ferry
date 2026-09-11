package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/store"
	"github.com/choffmann/ferry/internal/ticket"
)

func newTestServer(t *testing.T) (http.Handler, *chaos.MemoryStore) {
	t.Helper()
	cs := chaos.NewMemoryStore()
	h := NewRouter(Deps{
		Repo:       store.NewMemoryStore(time.Now().UTC()),
		Chaos:      cs,
		AdminToken: "test-token",
		Logger:     obs.NewLogger(io.Discard),
		Tickets:    testRenderer(t),
		Draw:       func() float64 { return 1 },
	})
	return h, cs
}

// The router gets its own minimal template so that these tests cover the route
// and not the wording of the shipped boarding pass.
func testRenderer(t *testing.T) *ticket.Renderer {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "ticket {{.Booking.ID}} {{.Connection.From.Name}}->{{.Connection.To.Name}}\n"
	if err := os.WriteFile(filepath.Join(dir, "tickets", "ticket.txt.tmpl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ticket.Load(dir)
	if err != nil {
		t.Fatalf("ticket.Load: %v", err)
	}
	return r
}

func openDepartureID(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/connections/FL-SO/departures", "")
	var ds []domain.Departure
	if err := json.Unmarshal(rec.Body.Bytes(), &ds); err != nil {
		t.Fatalf("departures: %v", err)
	}
	if len(ds) == 0 {
		t.Fatal("no departures on offer")
	}
	return ds[0].ID
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, r))
	return rec
}

func TestConnectionsAreListed(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/connections", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got []domain.Connection
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(got) != len(domain.Connections()) {
		t.Errorf("len = %d, want %d", len(got), len(domain.Connections()))
	}
}

func TestDeparturesOfAConnection(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/connections/FL-SO/departures", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got []domain.Departure
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no departures")
	}
	for _, d := range got {
		if d.ConnectionID != "FL-SO" {
			t.Fatalf("got a departure of %s", d.ConnectionID)
		}
	}
}

func TestDeparturesThatHaveLeftAreNotListed(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/connections/FL-SO/departures?from=2000-01-01T00:00:00Z", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got []domain.Departure
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no departures")
	}
	now := time.Now().UTC()
	for _, d := range got {
		if !domain.BookingOpen(d, now) {
			t.Errorf("departure %s has left and is still on offer", d.ID)
		}
	}
}

func TestDeparturesRejectAnUnparsableFrom(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/connections/FL-SO/departures?from=morgen", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestBookingRoundTrip(t *testing.T) {
	h, _ := newTestServer(t)

	list := do(t, h, http.MethodGet, "/connections/FL-SO/departures", "")
	var departures []domain.Departure
	if err := json.Unmarshal(list.Body.Bytes(), &departures); err != nil {
		t.Fatalf("departures: %v", err)
	}

	body := `{"departure_id":"` + departures[0].ID + `","passengers":2}`
	created := do(t, h, http.MethodPost, "/bookings", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", created.Code, created.Body)
	}
	var b domain.Booking
	if err := json.Unmarshal(created.Body.Bytes(), &b); err != nil {
		t.Fatalf("booking: %v", err)
	}
	if b.Status != domain.StatusConfirmed {
		t.Errorf("status = %q", b.Status)
	}

	read := do(t, h, http.MethodGet, "/bookings/"+b.ID, "")
	if read.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", read.Code, read.Body)
	}
}

func TestBookingErrors(t *testing.T) {
	h, _ := newTestServer(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"kein JSON", "{", http.StatusBadRequest},
		{"keine Abfahrt", `{"departure_id":"","passengers":1}`, http.StatusBadRequest},
		{"null Personen", `{"departure_id":"` + openDepartureID(t, h) + `","passengers":0}`, http.StatusBadRequest},
		{"unbekannte Abfahrt", `{"departure_id":"gibtsnicht","passengers":1}`, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/bookings", c.body)
			if rec.Code != c.want {
				t.Errorf("status = %d, want %d, body %s", rec.Code, c.want, rec.Body)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not JSON: %v", err)
			}
			if body["error"] == "" {
				t.Error("the error body has no message")
			}
		})
	}
}

func TestAFullDepartureAnswersWithConflict(t *testing.T) {
	h, _ := newTestServer(t)
	id := openDepartureID(t, h)

	for i := 0; i < domain.SeatsPerDeparture; i++ {
		rec := do(t, h, http.MethodPost, "/bookings", `{"departure_id":"`+id+`","passengers":1}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("booking %d: status %d, body %s", i, rec.Code, rec.Body)
		}
	}
	rec := do(t, h, http.MethodPost, "/bookings", `{"departure_id":"`+id+`","passengers":1}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestOverbookingSwitchAllowsBookingPastCapacity(t *testing.T) {
	h, _ := newTestServer(t)
	id := openDepartureID(t, h)

	for i := 0; i < domain.SeatsPerDeparture; i++ {
		rec := do(t, h, http.MethodPost, "/bookings", `{"departure_id":"`+id+`","passengers":1}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("booking %d: status %d, body %s", i, rec.Code, rec.Body)
		}
	}

	body := `{"departure_id":"` + id + `","passengers":1}`
	if rec := do(t, h, http.MethodPost, "/bookings", body); rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 before the switch is flipped", rec.Code)
	}

	patch := withToken(t, h, http.MethodPost, "/admin/chaos", `{"booking":{"allow_overbooking":true}}`, "test-token")
	if patch.Code != http.StatusOK {
		t.Fatalf("POST /admin/chaos: status %d, body %s", patch.Code, patch.Body)
	}

	if rec := do(t, h, http.MethodPost, "/bookings", body); rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201 after allow_overbooking was switched on", rec.Code)
	}
}

func TestUnknownBookingIsNotFound(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := do(t, h, http.MethodGet, "/bookings/gibtsnicht", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHealthzIsAlwaysUp(t *testing.T) {
	h, cs := newTestServer(t)
	fail := "fail"
	if _, err := cs.Apply(context.Background(), chaos.Patch{Readiness: &fail}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rec := do(t, h, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d although only readiness was switched off", rec.Code)
	}
}

func TestReadyzFollowsTheSwitch(t *testing.T) {
	h, cs := newTestServer(t)
	if rec := do(t, h, http.MethodGet, "/readyz", ""); rec.Code != http.StatusOK {
		t.Fatalf("readyz = %d before any chaos", rec.Code)
	}

	fail := "fail"
	if _, err := cs.Apply(context.Background(), chaos.Patch{Readiness: &fail}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rec := do(t, h, http.MethodGet, "/readyz", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz = %d, want 503", rec.Code)
	}
}

func TestTicketIsServedAsPlainText(t *testing.T) {
	h, _ := newTestServer(t)
	departureID := openDepartureID(t, h)

	rec := do(t, h, http.MethodPost, "/bookings",
		`{"departure_id":"`+departureID+`","passengers":2}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("booking status = %d, body %s", rec.Code, rec.Body)
	}
	var b domain.Booking
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("booking: %v", err)
	}

	rec = do(t, h, http.MethodGet, "/bookings/"+b.ID+"/ticket", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	want := "ticket " + b.ID + " Flensburg->Sønderborg\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestTicketForAnUnknownBookingIsNotFound(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/bookings/bk-999999/ticket", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the JSON error format: %s", rec.Body)
	}
	if body.Error != "booking not found" {
		t.Errorf("error = %q, want booking not found", body.Error)
	}
}

func TestVersionMirrorsTheBuildStamp(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got obs.BuildInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if got != obs.Version() {
		t.Errorf("build info = %+v, want %+v", got, obs.Version())
	}
}

func TestReadyzFailsWhenTheDatabaseDoesNot(t *testing.T) {
	cs := chaos.NewMemoryStore()
	h := NewRouter(Deps{
		Repo:       store.NewMemoryStore(time.Now().UTC()),
		Chaos:      cs,
		AdminToken: "test-token",
		Logger:     obs.NewLogger(io.Discard),
		Tickets:    testRenderer(t),
		Draw:       func() float64 { return 1 },
		Ping:       func(context.Context) error { return errors.New("connection refused") },
	})

	if rec := do(t, h, http.MethodGet, "/readyz", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz = %d although the database does not answer, want 503", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, the process is alive either way", rec.Code)
	}
}
