package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/choffmann/ferry/internal/domain"
)

// SeedResult counts the rows that were actually written. A second run reports
// zeroes, because it collides with everything that is already there.
type SeedResult struct {
	Connections int
	Departures  int
}

// Seed writes the connections and the timetable of the seven days that start
// with at's date, on at's own wall clock. Existing rows are left untouched, so
// booked seats survive and a later run rolls the window forward.
func Seed(ctx context.Context, pool *pgxpool.Pool, at time.Time) (SeedResult, error) {
	batch := &pgx.Batch{}
	for _, c := range domain.Connections() {
		batch.Queue(`
INSERT INTO connections (id, from_code, from_name, to_code, to_name, duration_minutes)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING`,
			c.ID, c.From.Code, c.From.Name, c.To.Code, c.To.Name, c.DurationMinutes)
	}
	for _, d := range domain.Departures(at) {
		batch.Queue(`
INSERT INTO departures (id, connection_id, departs_at, capacity)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO NOTHING`,
			d.ID, d.ConnectionID, d.DepartsAt, d.Capacity)
	}

	results := pool.SendBatch(ctx, batch)
	out, err := drain(results, batch.Len(), len(domain.Connections()))
	if closeErr := results.Close(); err == nil {
		err = closeErr
	}
	return out, err
}

// The first connections queries of the batch are the connections, the rest are
// departures, which is how the counts stay apart without a second round trip.
func drain(results pgx.BatchResults, queued, connections int) (SeedResult, error) {
	var out SeedResult
	for i := 0; i < queued; i++ {
		tag, err := results.Exec()
		if err != nil {
			return out, fmt.Errorf("seeding: %w", err)
		}
		if i < connections {
			out.Connections += int(tag.RowsAffected())
		} else {
			out.Departures += int(tag.RowsAffected())
		}
	}
	return out, nil
}
