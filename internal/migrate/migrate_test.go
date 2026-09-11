package migrate

import (
	"context"
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
