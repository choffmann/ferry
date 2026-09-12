package payments_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/payments"
)

func router() http.Handler {
	return payments.NewRouter(obs.NewLogger(&bytes.Buffer{}))
}

func TestChargeAnswersTwoHundredForADecline(t *testing.T) {
	body := `{"booking_id":"bk-000001","amount_cents":99999}`
	req := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var res payments.ChargeResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if res.Status != payments.Declined {
		t.Errorf("status = %q, want declined", res.Status)
	}
}

func TestChargeRejectsABodyItCannotRead(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestChargeRejectsAnAmountBelowOne(t *testing.T) {
	body := `{"booking_id":"bk-000001","amount_cents":0}`
	req := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTheOperationalEndpointsAnswer(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/version"} {
		rec := httptest.NewRecorder()
		router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200", path, rec.Code)
		}
	}
}
