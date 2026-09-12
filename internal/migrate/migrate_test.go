package migrate

import (
	"context"
	"sort"
	"testing"

	"github.com/choffmann/ferry/internal/pgtest"
)

func TestApplyCreatesTheTables(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	applied, err := Apply(ctx, pool)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(applied) == 0 {
		t.Fatal("Apply reported no migrations on an empty schema")
	}

	for _, table := range []string{"connections", "departures", "bookings"} {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Errorf("table %s is missing: %v", table, err)
		}
	}
}

func TestApplyLeavesNothingToDoOnASecondRun(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	if _, err := Apply(ctx, pool); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	again, err := Apply(ctx, pool)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("the second run applied %v, want nothing", again)
	}
}

func TestApplyRunsTheMigrationsInFileOrder(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	applied, err := Apply(ctx, pool)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(applied) < 2 {
		t.Fatalf("applied %v, want both migrations", applied)
	}
	if !sort.SliceIsSorted(applied, func(i, j int) bool { return applied[i] < applied[j] }) {
		t.Errorf("applied %v, want them in file order", applied)
	}

	// The column only exists if 0002 ran after 0001 created the table.
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(amount_cents) FROM bookings").Scan(&n); err != nil {
		t.Errorf("amount_cents is missing: %v", err)
	}
}
