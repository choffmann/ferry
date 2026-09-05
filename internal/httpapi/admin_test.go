package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/choffmann/ferry/internal/chaos"
)

func withToken(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestChaosEndpointsNeedTheToken(t *testing.T) {
	h, _ := newTestServer(t)

	cases := []struct {
		method, token string
		want          int
	}{
		{http.MethodGet, "", http.StatusUnauthorized},
		{http.MethodGet, "falsch", http.StatusUnauthorized},
		{http.MethodGet, "test-token", http.StatusOK},
		{http.MethodPost, "", http.StatusUnauthorized},
		{http.MethodDelete, "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		body := ""
		if c.method == http.MethodPost {
			body = "{}"
		}
		rec := withToken(t, h, c.method, "/admin/chaos", body, c.token)
		if rec.Code != c.want {
			t.Errorf("%s with token %q = %d, want %d", c.method, c.token, rec.Code, c.want)
		}
	}
}

func TestPostChaosAppliesPartially(t *testing.T) {
	h, _ := newTestServer(t)

	first := withToken(t, h, http.MethodPost, "/admin/chaos", `{"http":{"latency_ms":120}}`, "test-token")
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", first.Code, first.Body)
	}

	second := withToken(t, h, http.MethodPost, "/admin/chaos", `{"readiness":"fail"}`, "test-token")
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", second.Code, second.Body)
	}

	var got chaos.State
	if err := json.Unmarshal(second.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if got.HTTP.LatencyMS != 120 {
		t.Errorf("LatencyMS = %d, the second POST did not mention it", got.HTTP.LatencyMS)
	}
	if got.Readiness != "fail" {
		t.Errorf("Readiness = %q, want fail", got.Readiness)
	}
}

func TestPostChaosRejectsBrokenJSON(t *testing.T) {
	h, _ := newTestServer(t)
	rec := withToken(t, h, http.MethodPost, "/admin/chaos", "{", "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPostChaosRejectsAMisspelledField(t *testing.T) {
	h, _ := newTestServer(t)

	before := withToken(t, h, http.MethodGet, "/admin/chaos", "", "test-token")

	rec := withToken(t, h, http.MethodPost, "/admin/chaos", `{"http":{"latency":500}}`, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	after := withToken(t, h, http.MethodGet, "/admin/chaos", "", "test-token")
	if after.Body.String() != before.Body.String() {
		t.Errorf("state changed after a rejected patch: before %s, after %s", before.Body, after.Body)
	}
}

func TestDeleteChaosResets(t *testing.T) {
	h, _ := newTestServer(t)

	withToken(t, h, http.MethodPost, "/admin/chaos", `{"readiness":"fail"}`, "test-token")
	if rec := withToken(t, h, http.MethodGet, "/readyz", "", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz = %d, the switch did not take", rec.Code)
	}

	if rec := withToken(t, h, http.MethodDelete, "/admin/chaos", "", "test-token"); rec.Code != http.StatusOK {
		t.Fatalf("delete = %d", rec.Code)
	}
	if rec := withToken(t, h, http.MethodGet, "/readyz", "", ""); rec.Code != http.StatusOK {
		t.Errorf("readyz = %d after the reset, want 200", rec.Code)
	}
}
