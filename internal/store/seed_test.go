package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/migrate"
	"github.com/choffmann/ferry/internal/pgtest"
)

func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := pgtest.Pool(t)
	if _, err := migrate.Apply(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func TestSeedWritesTheTimetable(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	at := seedDay()

	got, err := Seed(ctx, pool, at)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if want := len(domain.Connections()); got.Connections != want {
		t.Errorf("reported %d connections, want %d", got.Connections, want)
	}
	if want := len(domain.Departures(at)); got.Departures != want {
		t.Errorf("reported %d departures, want %d", got.Departures, want)
	}

	var departures int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM departures").Scan(&departures); err != nil {
		t.Fatalf("counting departures: %v", err)
	}
	if want := len(domain.Departures(at)); departures != want {
		t.Errorf("%d departures in the database, want %d", departures, want)
	}
}

func TestSeedLeavesBookedSeatsAlone(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	at := seedDay()

	if _, err := Seed(ctx, pool, at); err != nil {
		t.Fatalf("first Seed: %v", err)
	}

	id := domain.Departures(at)[0].ID
	if _, err := pool.Exec(ctx,
		"UPDATE departures SET booked = 7 WHERE id = $1", id); err != nil {
		t.Fatalf("booking seats: %v", err)
	}

	got, err := Seed(ctx, pool, at)
	if err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if got.Departures != 0 {
		t.Errorf("the second run wrote %d departures, want 0", got.Departures)
	}

	var booked int
	if err := pool.QueryRow(ctx,
		"SELECT booked FROM departures WHERE id = $1", id).Scan(&booked); err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if booked != 7 {
		t.Errorf("booked = %d after a second seed, want 7", booked)
	}
}
