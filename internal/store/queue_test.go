package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/pgtest"
)

func bookOne(t *testing.T, q Repository, passengers int) domain.Booking {
	t.Helper()
	dep, err := q.Departures(context.Background(), "FL-SO", time.Time{})
	if err != nil || len(dep) == 0 {
		t.Fatalf("Departures: %v (%d)", err, len(dep))
	}
	b, err := q.Book(context.Background(),
		domain.BookingRequest{DepartureID: dep[0].ID, Passengers: passengers},
		domain.BookOptions{})
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	return b
}

func runQueueContract(t *testing.T, newStore func(seed time.Time, now func() time.Time) interface {
	Repository
	Queue
}) {
	t.Helper()
	ctx := context.Background()

	t.Run("a new booking is pending and carries its fare", func(t *testing.T) {
		s := newStore(seedDay(), fixed(duringSeed()))
		b := bookOne(t, s, 3)
		if b.Status != domain.StatusPending {
			t.Errorf("status = %q, want %q", b.Status, domain.StatusPending)
		}
		if b.AmountCents != domain.FareCents(3) {
			t.Errorf("amount = %d, want %d", b.AmountCents, domain.FareCents(3))
		}
	})

	t.Run("claim hands out an open booking once", func(t *testing.T) {
		s := newStore(seedDay(), fixed(duringSeed()))
		b := bookOne(t, s, 1)

		first, err := s.Claim(ctx, 10)
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if len(first) != 1 || first[0].ID != b.ID {
			t.Fatalf("first claim = %+v, want the one booking %s", first, b.ID)
		}

		second, err := s.Claim(ctx, 10)
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if len(second) != 0 {
			t.Errorf("second claim = %+v, want nothing", second)
		}
	})

	t.Run("release puts a claimed booking back", func(t *testing.T) {
		s := newStore(seedDay(), fixed(duringSeed()))
		b := bookOne(t, s, 1)
		if _, err := s.Claim(ctx, 10); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if err := s.Release(ctx, b.ID); err != nil {
			t.Fatalf("Release: %v", err)
		}
		again, err := s.Claim(ctx, 10)
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if len(again) != 1 {
			t.Errorf("claim after release = %d bookings, want 1", len(again))
		}
	})

	t.Run("confirming keeps the seats", func(t *testing.T) {
		s := newStore(seedDay(), fixed(duringSeed()))
		b := bookOne(t, s, 4)
		before := seatsTaken(t, s, b.DepartureID)

		if _, err := s.Claim(ctx, 10); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		err := s.Settle(ctx, domain.Settlement{
			BookingID: b.ID, Status: domain.StatusConfirmed, Reference: "pay-1",
		})
		if err != nil {
			t.Fatalf("Settle: %v", err)
		}

		got, err := s.Booking(ctx, b.ID)
		if err != nil {
			t.Fatalf("Booking: %v", err)
		}
		if got.Status != domain.StatusConfirmed {
			t.Errorf("status = %q, want confirmed", got.Status)
		}
		if got.PaymentRef != "pay-1" {
			t.Errorf("payment_ref = %q, want pay-1", got.PaymentRef)
		}
		if after := seatsTaken(t, s, b.DepartureID); after != before {
			t.Errorf("seats taken = %d, want %d", after, before)
		}
	})

	t.Run("failing gives the seats back", func(t *testing.T) {
		s := newStore(seedDay(), fixed(duringSeed()))
		b := bookOne(t, s, 4)
		before := seatsTaken(t, s, b.DepartureID)

		if _, err := s.Claim(ctx, 10); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		err := s.Settle(ctx, domain.Settlement{
			BookingID: b.ID, Status: domain.StatusFailed, Reason: "no",
		})
		if err != nil {
			t.Fatalf("Settle: %v", err)
		}

		got, err := s.Booking(ctx, b.ID)
		if err != nil {
			t.Fatalf("Booking: %v", err)
		}
		if got.Status != domain.StatusFailed || got.FailureReason != "no" {
			t.Errorf("booking = %+v, want failed with a reason", got)
		}
		if after := seatsTaken(t, s, b.DepartureID); after != before-4 {
			t.Errorf("seats taken = %d, want %d", after, before-4)
		}
	})

	// A worker that dies after the charge leaves its claim behind. Without this
	// the booking would never be settled by anyone.
	t.Run("a stale claim is handed out again", func(t *testing.T) {
		at := duringSeed()
		s := newStore(seedDay(), func() time.Time { return at })
		b := bookOne(t, s, 1)

		if _, err := s.Claim(ctx, 10); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		at = at.Add(2 * time.Minute)

		again, err := s.Claim(ctx, 10)
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if len(again) != 1 || again[0].ID != b.ID {
			t.Errorf("claim after the stale window = %+v, want %s", again, b.ID)
		}
	})

	t.Run("settling twice is refused", func(t *testing.T) {
		s := newStore(seedDay(), fixed(duringSeed()))
		b := bookOne(t, s, 2)
		if _, err := s.Claim(ctx, 10); err != nil {
			t.Fatalf("Claim: %v", err)
		}
		settle := domain.Settlement{BookingID: b.ID, Status: domain.StatusFailed, Reason: "no"}
		if err := s.Settle(ctx, settle); err != nil {
			t.Fatalf("first Settle: %v", err)
		}
		if err := s.Settle(ctx, settle); err == nil {
			t.Error("the second Settle went through, want an error")
		}
	})
}

func seatsTaken(t *testing.T, r Repository, departureID string) int {
	t.Helper()
	d, err := r.Departure(context.Background(), departureID)
	if err != nil {
		t.Fatalf("Departure: %v", err)
	}
	return d.Booked
}

func TestFakeQueueContract(t *testing.T) {
	runQueueContract(t, func(seed time.Time, now func() time.Time) interface {
		Repository
		Queue
	} {
		return newFake(seed, now)
	})
}

func TestPostgresQueueContract(t *testing.T) {
	pgtest.URL(t)

	runQueueContract(t, func(seed time.Time, now func() time.Time) interface {
		Repository
		Queue
	} {
		pool := migratedPool(t)
		if _, err := Seed(context.Background(), pool, seed); err != nil {
			t.Fatalf("Seed: %v", err)
		}
		return newPostgresStore(pool, seed.Location(), now)
	})
}

// SKIP LOCKED is what the whole worker design rests on: two claimers must never
// get the same row. The Fake serialises Claim under its mutex, so this is
// Postgres-only rather than part of the shared contract.
func TestConcurrentClaimsGetDisjointBookings(t *testing.T) {
	pgtest.URL(t)
	ctx := context.Background()

	pool := migratedPool(t)
	seed := seedDay()
	if _, err := Seed(ctx, pool, seed); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	s := newPostgresStore(pool, seed.Location(), fixed(duringSeed()))

	for i := 0; i < 6; i++ {
		bookOne(t, s, 1)
	}

	var mu sync.Mutex
	var first, second []domain.Booking
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		got, err := s.Claim(ctx, 3)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			t.Errorf("first Claim: %v", err)
			return
		}
		first = got
	}()
	go func() {
		defer wg.Done()
		got, err := s.Claim(ctx, 3)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			t.Errorf("second Claim: %v", err)
			return
		}
		second = got
	}()
	wg.Wait()

	seen := map[string]bool{}
	for _, b := range first {
		seen[b.ID] = true
	}
	for _, b := range second {
		if seen[b.ID] {
			t.Errorf("booking %s was claimed by both calls", b.ID)
		}
		seen[b.ID] = true
	}
	if len(seen) != 6 {
		t.Errorf("the two claims covered %d distinct bookings, want 6", len(seen))
	}
}
