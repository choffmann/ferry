package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/store"
	"github.com/choffmann/ferry/internal/ticket"
)

type Deps struct {
	Repo       store.Repository
	Chaos      chaos.Store
	AdminToken string
	Logger     *slog.Logger
	Tickets    *ticket.Renderer
	// Draw supplies the random number for the error-rate switch. Tests pin it.
	Draw func() float64
}

// NewRouter is the single place where routes are registered, so it must not be
// spread across files.
func NewRouter(d Deps) http.Handler {
	if d.Draw == nil {
		d.Draw = defaultDraw
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /connections", d.listConnections)
	mux.HandleFunc("GET /connections/{id}/departures", d.listDepartures)
	mux.HandleFunc("POST /bookings", d.createBooking)
	mux.HandleFunc("GET /bookings/{id}", d.getBooking)
	mux.HandleFunc("GET /bookings/{id}/ticket", d.bookingTicket)

	mux.HandleFunc("GET /healthz", d.healthz)
	mux.HandleFunc("GET /readyz", d.readyz)
	mux.HandleFunc("GET /version", d.version)

	mux.HandleFunc("GET /admin/chaos", d.getChaos)
	mux.HandleFunc("POST /admin/chaos", d.postChaos)
	mux.HandleFunc("DELETE /admin/chaos", d.deleteChaos)

	var h http.Handler = mux
	h = chaosMiddleware(d.Chaos, d.Draw)(h)
	h = logging(d.Logger)(h)
	h = requestID(h)
	h = recoverer(d.Logger)(h)
	return h
}
