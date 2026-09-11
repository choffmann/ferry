package store

import (
	"context"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/pgtest"
)

func TestPostgresStoreSatisfiesTheContract(t *testing.T) {
	pgtest.URL(t)

	runRepositoryContract(t, func(seed time.Time, now func() time.Time) Repository {
		pool := migratedPool(t)
		if _, err := Seed(context.Background(), pool, seed); err != nil {
			t.Fatalf("Seed: %v", err)
		}
		return newPostgresStore(pool, seed.Location(), now)
	})
}

// Postgres hands timestamps back in UTC. Without converting them the local time
// of the timetable would disappear from the API without a test noticing.
func TestPostgresStoreReturnsDeparturesOnTheTimetableClock(t *testing.T) {
	pgtest.URL(t)

	zone, err := domain.LoadZone()
	if err != nil {
		t.Fatalf("LoadZone: %v", err)
	}

	ctx := context.Background()
	pool := migratedPool(t)
	summer := time.Date(2026, 7, 1, 0, 0, 0, 0, zone)
	if _, err := Seed(ctx, pool, summer); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	r := newPostgresStore(pool, zone, fixed(summer.Add(5*time.Hour)))
	ds, err := r.Departures(ctx, "FL-SO", time.Time{})
	if err != nil {
		t.Fatalf("Departures: %v", err)
	}
	if len(ds) == 0 {
		t.Fatal("no departures for FL-SO")
	}

	got := ds[0].DepartsAt.Format(time.RFC3339)
	if want := "2026-07-01T06:00:00+02:00"; got != want {
		t.Errorf("first departure = %s, want %s", got, want)
	}
}
