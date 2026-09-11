package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens the pool and proves once that the database answers. It does not
// retry: a database that is not there yet is a problem of the topology, and the
// process says so instead of hiding it behind a wait loop.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("database url is not usable: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database is not reachable: %w", err)
	}
	return pool, nil
}
