package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

type MemoryStore struct {
	mu           sync.Mutex
	connections  []domain.Connection
	departures   map[string]domain.Departure
	departureIDs []string
	bookings     map[string]domain.Booking
	nextBooking  int
	now          func() time.Time
}

func NewMemoryStore(now time.Time) *MemoryStore {
	return newMemoryStore(now, func() time.Time { return time.Now().UTC() })
}

// The seed date and the clock are separate: the timetable is built once for a
// day, while the boarding deadline is checked against the moment of the request.
func newMemoryStore(seed time.Time, now func() time.Time) *MemoryStore {
	s := &MemoryStore{
		connections: domain.Connections(),
		departures:  map[string]domain.Departure{},
		bookings:    map[string]domain.Booking{},
		now:         now,
	}
	for _, d := range domain.Departures(seed) {
		s.departures[d.ID] = d
		s.departureIDs = append(s.departureIDs, d.ID)
	}
	return s
}

func (s *MemoryStore) Connections(ctx context.Context) ([]domain.Connection, error) {
	return append([]domain.Connection(nil), s.connections...), nil
}

func (s *MemoryStore) Departures(ctx context.Context, connectionID string, from time.Time) ([]domain.Departure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	var out []domain.Departure
	for _, id := range s.departureIDs {
		d := s.departures[id]
		if d.ConnectionID != connectionID {
			continue
		}
		if !domain.BookingOpen(d, now) {
			continue
		}
		if !from.IsZero() && d.DepartsAt.Before(from) {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DepartsAt.Before(out[j].DepartsAt) })
	return out, nil
}

func (s *MemoryStore) Departure(ctx context.Context, id string) (domain.Departure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.departures[id]
	if !ok {
		return domain.Departure{}, fmt.Errorf("%w: departure %s", ErrNotFound, id)
	}
	return d, nil
}

func (s *MemoryStore) Booking(ctx context.Context, id string) (domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bookings[id]
	if !ok {
		return domain.Booking{}, fmt.Errorf("%w: booking %s", ErrNotFound, id)
	}
	return b, nil
}

func (s *MemoryStore) Book(ctx context.Context, req domain.BookingRequest, opts domain.BookOptions) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.departures[req.DepartureID]
	if !ok {
		return domain.Booking{}, fmt.Errorf("%w: departure %s", ErrNotFound, req.DepartureID)
	}
	if err := domain.CheckBookable(d, s.now(), req.Passengers, opts); err != nil {
		return domain.Booking{}, err
	}

	d.Booked += req.Passengers
	s.departures[d.ID] = d

	s.nextBooking++
	b := domain.Booking{
		ID:          fmt.Sprintf("bk-%06d", s.nextBooking),
		DepartureID: d.ID,
		Passengers:  req.Passengers,
		Status:      domain.StatusConfirmed,
		CreatedAt:   s.now(),
	}
	s.bookings[b.ID] = b
	return b, nil
}
