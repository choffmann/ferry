package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrSoldOut        = errors.New("no seats left")
	ErrInvalidRequest = errors.New("invalid booking request")
	ErrBookingClosed  = errors.New("booking has closed for this departure")
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

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusFailed    BookingStatus = "failed"
)

type Booking struct {
	ID          string        `json:"id"`
	DepartureID string        `json:"departure_id"`
	Passengers  int           `json:"passengers"`
	AmountCents int           `json:"amount_cents"`
	Status      BookingStatus `json:"status"`
	// Both are empty until the worker has settled the booking, and only one of
	// them is ever filled.
	PaymentRef    string    `json:"payment_ref,omitempty"`
	FailureReason string    `json:"failure_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Settlement is the outcome the worker writes back after the payment service
// has answered.
type Settlement struct {
	BookingID string
	Status    BookingStatus
	Reference string
	Reason    string
}

type BookingRequest struct {
	DepartureID string `json:"departure_id"`
	Passengers  int    `json:"passengers"`
}

const maxPassengersPerBooking = 9

func (r BookingRequest) Validate() error {
	if r.DepartureID == "" {
		return fmt.Errorf("%w: departure_id missing", ErrInvalidRequest)
	}
	if r.Passengers < 1 || r.Passengers > maxPassengersPerBooking {
		return fmt.Errorf("%w: passengers must be between 1 and %d",
			ErrInvalidRequest, maxPassengersPerBooking)
	}
	return nil
}

// BookOptions carries the booking policy. AllowOverbooking is what the chaos
// switch sets, which keeps the persistence layer free of any chaos concept.
type BookOptions struct {
	AllowOverbooking bool
}

// BookingOpen reports whether a departure still takes bookings. Boarding closes
// a while before it leaves, so a sailing that has gone is no longer on offer.
func BookingOpen(d Departure, now time.Time) bool {
	return now.Before(d.DepartsAt.Add(-BoardingClosesBefore))
}

// CheckBookable is the booking invariant. It lives here and not in the store so
// that the Postgres implementation in v0.3.0 does not restate it. Overbooking
// lifts the capacity limit, never the boarding deadline.
func CheckBookable(d Departure, now time.Time, want int, opts BookOptions) error {
	if !BookingOpen(d, now) {
		return ErrBookingClosed
	}
	if opts.AllowOverbooking {
		return nil
	}
	if want > d.SeatsAvailable() {
		return ErrSoldOut
	}
	return nil
}

func FareCents(passengers int) int {
	return passengers * FareCentsPerPassenger
}
