package store

import (
	"context"
	"errors"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

var ErrNotFound = errors.New("not found")

// Repository is the persistence port. Book is a single method on purpose: the
// seat accounting has to happen inside one critical section, otherwise two
// concurrent requests both pass the capacity check.
type Repository interface {
	Connections(ctx context.Context) ([]domain.Connection, error)
	Departures(ctx context.Context, connectionID string, from time.Time) ([]domain.Departure, error)
	Departure(ctx context.Context, id string) (domain.Departure, error)
	Booking(ctx context.Context, id string) (domain.Booking, error)
	Book(ctx context.Context, req domain.BookingRequest, opts domain.BookOptions) (domain.Booking, error)
}

// Queue is the port the worker settles bookings through. Claim marks what it
// hands out, so a second worker gets different rows.
type Queue interface {
	Claim(ctx context.Context, limit int) ([]domain.Booking, error)
	Settle(ctx context.Context, s domain.Settlement) error
	Release(ctx context.Context, bookingID string) error
}
