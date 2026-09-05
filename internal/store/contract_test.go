package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

func seedTime() time.Time {
	return time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
}

func runRepositoryContract(t *testing.T, newRepo func(now time.Time) Repository) {
	t.Helper()

	t.Run("connections are returned", func(t *testing.T) {
		r := newRepo(seedTime())
		got, err := r.Connections(context.Background())
		if err != nil {
			t.Fatalf("Connections: %v", err)
		}
		if len(got) != len(domain.Connections()) {
			t.Errorf("len = %d, want %d", len(got), len(domain.Connections()))
		}
	})

	t.Run("departures are filtered by connection and start time", func(t *testing.T) {
		r := newRepo(seedTime())
		all, err := r.Departures(context.Background(), "FL-SO", time.Time{})
		if err != nil {
			t.Fatalf("Departures: %v", err)
		}
		if len(all) == 0 {
			t.Fatal("no departures for FL-SO")
		}
		for _, d := range all {
			if d.ConnectionID != "FL-SO" {
				t.Fatalf("got a departure of %s", d.ConnectionID)
			}
		}

		cut := all[len(all)/2].DepartsAt
		later, err := r.Departures(context.Background(), "FL-SO", cut)
		if err != nil {
			t.Fatalf("Departures with from: %v", err)
		}
		if len(later) >= len(all) {
			t.Errorf("filtering by from returned %d of %d", len(later), len(all))
		}
		for _, d := range later {
			if d.DepartsAt.Before(cut) {
				t.Fatalf("departure %s is before the cut", d.ID)
			}
		}
	})

	t.Run("unknown ids report not found", func(t *testing.T) {
		r := newRepo(seedTime())
		if _, err := r.Departure(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Departure = %v, want ErrNotFound", err)
		}
		if _, err := r.Booking(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Booking = %v, want ErrNotFound", err)
		}
	})

	t.Run("booking reduces the free seats and can be read back", func(t *testing.T) {
		r := newRepo(seedTime())
		d := firstDeparture(t, r)

		b, err := r.Book(context.Background(),
			domain.BookingRequest{DepartureID: d.ID, Passengers: 3}, domain.BookOptions{})
		if err != nil {
			t.Fatalf("Book: %v", err)
		}
		if b.Status != domain.StatusConfirmed {
			t.Errorf("status = %q, want %q", b.Status, domain.StatusConfirmed)
		}

		again, err := r.Booking(context.Background(), b.ID)
		if err != nil {
			t.Fatalf("Booking: %v", err)
		}
		if again != b {
			t.Errorf("read back %+v, wrote %+v", again, b)
		}

		after, err := r.Departure(context.Background(), d.ID)
		if err != nil {
			t.Fatalf("Departure: %v", err)
		}
		if after.Booked != 3 {
			t.Errorf("booked = %d, want 3", after.Booked)
		}
	})

	t.Run("an invalid request is rejected before anything is written", func(t *testing.T) {
		r := newRepo(seedTime())
		d := firstDeparture(t, r)

		if _, err := r.Book(context.Background(),
			domain.BookingRequest{DepartureID: d.ID, Passengers: 0}, domain.BookOptions{}); !errors.Is(err, domain.ErrInvalidRequest) {
			t.Errorf("Book with zero passengers = %v, want ErrInvalidRequest", err)
		}

		after, err := r.Departure(context.Background(), d.ID)
		if err != nil {
			t.Fatalf("Departure: %v", err)
		}
		if after.Booked != 0 {
			t.Errorf("booked = %d after a rejected request, want 0", after.Booked)
		}
	})

	t.Run("a full departure is sold out", func(t *testing.T) {
		r := newRepo(seedTime())
		d := firstDeparture(t, r)

		for i := 0; i < domain.SeatsPerDeparture; i++ {
			if _, err := r.Book(context.Background(),
				domain.BookingRequest{DepartureID: d.ID, Passengers: 1}, domain.BookOptions{}); err != nil {
				t.Fatalf("booking seat %d: %v", i, err)
			}
		}
		_, err := r.Book(context.Background(),
			domain.BookingRequest{DepartureID: d.ID, Passengers: 1}, domain.BookOptions{})
		if !errors.Is(err, domain.ErrSoldOut) {
			t.Errorf("Book on a full departure = %v, want ErrSoldOut", err)
		}
	})

	t.Run("overbooking is allowed when the option says so", func(t *testing.T) {
		r := newRepo(seedTime())
		d := firstDeparture(t, r)

		for i := 0; i < domain.SeatsPerDeparture; i++ {
			if _, err := r.Book(context.Background(),
				domain.BookingRequest{DepartureID: d.ID, Passengers: 1}, domain.BookOptions{}); err != nil {
				t.Fatalf("booking seat %d: %v", i, err)
			}
		}
		if _, err := r.Book(context.Background(),
			domain.BookingRequest{DepartureID: d.ID, Passengers: 2},
			domain.BookOptions{AllowOverbooking: true}); err != nil {
			t.Fatalf("Book with overbooking allowed: %v", err)
		}

		after, err := r.Departure(context.Background(), d.ID)
		if err != nil {
			t.Fatalf("Departure: %v", err)
		}
		if after.Booked != domain.SeatsPerDeparture+2 {
			t.Errorf("booked = %d, want %d", after.Booked, domain.SeatsPerDeparture+2)
		}
	})

	t.Run("bookings are stamped with the current time, not the seed time", func(t *testing.T) {
		r := newRepo(seedTime())
		d := firstDeparture(t, r)

		first, err := r.Book(context.Background(),
			domain.BookingRequest{DepartureID: d.ID, Passengers: 1}, domain.BookOptions{})
		if err != nil {
			t.Fatalf("Book: %v", err)
		}

		time.Sleep(5 * time.Millisecond)

		second, err := r.Book(context.Background(),
			domain.BookingRequest{DepartureID: d.ID, Passengers: 1}, domain.BookOptions{})
		if err != nil {
			t.Fatalf("Book: %v", err)
		}

		if first.CreatedAt.Equal(second.CreatedAt) {
			t.Errorf("both bookings have CreatedAt %v", first.CreatedAt)
		}
		if first.CreatedAt.Equal(seedTime()) || second.CreatedAt.Equal(seedTime()) {
			t.Error("CreatedAt equals the seed time, want the current time")
		}
	})

	// The reason Book is one method instead of read, check, write. Splitting it
	// makes this test red.
	t.Run("concurrent bookings never exceed the capacity", func(t *testing.T) {
		r := newRepo(seedTime())
		d := firstDeparture(t, r)

		const attempts = 100
		var wg sync.WaitGroup
		results := make([]error, attempts)
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, results[i] = r.Book(context.Background(),
					domain.BookingRequest{DepartureID: d.ID, Passengers: 1}, domain.BookOptions{})
			}(i)
		}
		wg.Wait()

		var ok, soldOut int
		for _, err := range results {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, domain.ErrSoldOut):
				soldOut++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if ok != domain.SeatsPerDeparture {
			t.Errorf("%d bookings succeeded, want %d", ok, domain.SeatsPerDeparture)
		}
		if soldOut != attempts-domain.SeatsPerDeparture {
			t.Errorf("%d were sold out, want %d", soldOut, attempts-domain.SeatsPerDeparture)
		}

		after, err := r.Departure(context.Background(), d.ID)
		if err != nil {
			t.Fatalf("Departure: %v", err)
		}
		if after.Booked != domain.SeatsPerDeparture {
			t.Errorf("booked = %d, want exactly %d", after.Booked, domain.SeatsPerDeparture)
		}
	})
}

func firstDeparture(t *testing.T, r Repository) domain.Departure {
	t.Helper()
	ds, err := r.Departures(context.Background(), "FL-SO", time.Time{})
	if err != nil {
		t.Fatalf("Departures: %v", err)
	}
	if len(ds) == 0 {
		t.Fatal("no departures to book")
	}
	return ds[0]
}
