package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/store"
)

func (d Deps) listConnections(w http.ResponseWriter, r *http.Request) {
	cs, err := d.Repo.Connections(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "connections could not be read")
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (d Deps) listDepartures(w http.ResponseWriter, r *http.Request) {
	var from time.Time
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "from must be an RFC3339 timestamp")
			return
		}
		from = parsed
	}

	ds, err := d.Repo.Departures(r.Context(), r.PathValue("id"), from)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "departures could not be read")
		return
	}
	if ds == nil {
		ds = []domain.Departure{}
	}
	writeJSON(w, http.StatusOK, ds)
}

func (d Deps) createBooking(w http.ResponseWriter, r *http.Request) {
	var req domain.BookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "request body is not valid JSON")
		return
	}

	state, err := d.Chaos.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "chaos state not readable")
		return
	}
	opts := domain.BookOptions{AllowOverbooking: chaos.AllowOverbooking(state)}

	b, err := d.Repo.Book(r.Context(), req, opts)
	switch {
	case err == nil:
		obs.LoggerFrom(r.Context()).Info("booking created",
			"booking_id", b.ID, "departure_id", b.DepartureID, "passengers", b.Passengers)
		writeJSON(w, http.StatusCreated, b)
	case errors.Is(err, domain.ErrInvalidRequest):
		writeError(w, r, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "departure not found")
	case errors.Is(err, domain.ErrSoldOut):
		writeError(w, r, http.StatusConflict, "departure is sold out")
	case errors.Is(err, domain.ErrBookingClosed):
		writeError(w, r, http.StatusConflict, "departure is closed for booking")
	default:
		writeError(w, r, http.StatusInternalServerError, "booking failed")
	}
}

func (d Deps) getBooking(w http.ResponseWriter, r *http.Request) {
	b, err := d.Repo.Booking(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "booking not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "booking could not be read")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (d Deps) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (d Deps) readyz(w http.ResponseWriter, r *http.Request) {
	s, err := d.Chaos.Get(r.Context())
	if err != nil || !chaos.Ready(s) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (d Deps) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, obs.Version())
}
