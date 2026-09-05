package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrSoldOut        = errors.New("keine freien Plätze mehr")
	ErrInvalidRequest = errors.New("ungültige Buchungsanfrage")
)

type Port struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Connection struct {
	ID              string `json:"id"`
	From            Port   `json:"from"`
	To              Port   `json:"to"`
	DurationMinutes int    `json:"duration_minutes"`
}

type Departure struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	DepartsAt    time.Time `json:"departs_at"`
	Capacity     int       `json:"capacity"`
	Booked       int       `json:"booked"`
}

func (d Departure) SeatsAvailable() int {
	return d.Capacity - d.Booked
}

type BookingStatus string

const StatusConfirmed BookingStatus = "confirmed"

type Booking struct {
	ID          string        `json:"id"`
	DepartureID string        `json:"departure_id"`
	Passengers  int           `json:"passengers"`
	Status      BookingStatus `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
}

type BookingRequest struct {
	DepartureID string `json:"departure_id"`
	Passengers  int    `json:"passengers"`
}

const maxPassengersPerBooking = 9

func (r BookingRequest) Validate() error {
	if r.DepartureID == "" {
		return fmt.Errorf("%w: departure_id fehlt", ErrInvalidRequest)
	}
	if r.Passengers < 1 || r.Passengers > maxPassengersPerBooking {
		return fmt.Errorf("%w: passengers muss zwischen 1 und %d liegen",
			ErrInvalidRequest, maxPassengersPerBooking)
	}
	return nil
}

// BookOptions carries the booking policy. AllowOverbooking is what the chaos
// switch sets, which keeps the persistence layer free of any chaos concept.
type BookOptions struct {
	AllowOverbooking bool
}

// CheckSeats is the capacity invariant. It lives here and not in the store so
// that the Postgres implementation in v0.3.0 does not restate it.
func CheckSeats(d Departure, want int, opts BookOptions) error {
	if opts.AllowOverbooking {
		return nil
	}
	if want > d.SeatsAvailable() {
		return ErrSoldOut
	}
	return nil
}
