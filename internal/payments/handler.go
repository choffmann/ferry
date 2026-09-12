package payments

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/choffmann/ferry/internal/httpmw"
	"github.com/choffmann/ferry/internal/obs"
)

func NewRouter(l *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /charge", charge)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpmw.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		httpmw.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		httpmw.WriteJSON(w, http.StatusOK, obs.Version())
	})

	var h http.Handler = mux
	h = httpmw.Logging(l)(h)
	h = httpmw.RequestID(h)
	h = httpmw.Recoverer(l)(h)
	return h
}

// A decline is a valid answer about a payment and comes with 200. Only a
// request the service cannot act on at all is a 400.
func charge(w http.ResponseWriter, r *http.Request) {
	var req ChargeRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpmw.WriteError(w, r, http.StatusBadRequest, "request body is not valid JSON")
		return
	}
	if req.BookingID == "" {
		httpmw.WriteError(w, r, http.StatusBadRequest, "booking_id missing")
		return
	}
	if req.AmountCents < 1 {
		httpmw.WriteError(w, r, http.StatusBadRequest, "amount_cents must be positive")
		return
	}
	httpmw.WriteJSON(w, http.StatusOK, Decide(req))
}
