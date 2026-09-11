// Package migrate applies the embedded SQL migrations. Each file runs once, in
// the order of its name, inside its own transaction.
package migrate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/choffmann/ferry/migrations"
)

// Two runners started at the same time would otherwise interleave their files.
const advisoryLockKey = 6731254908113

const createVersionTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Apply returns the versions it applied, in order. An empty result means the
// database was already up to date.
func Apply(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring a connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return nil, fmt.Errorf("taking the migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx),
			"SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	if _, err := conn.Exec(ctx, createVersionTable); err != nil {
		return nil, fmt.Errorf("creating schema_migrations: %w", err)
	}

	done, err := appliedVersions(ctx, conn)
	if err != nil {
		return nil, err
	}

	available, err := available()
	if err != nil {
		return nil, err
	}

	var applied []string
	for _, version := range available {
		if done[version] {
			continue
		}
		body, err := migrations.FS.ReadFile(version)
		if err != nil {
			return applied, fmt.Errorf("reading %s: %w", version, err)
		}
		if err := applyOne(ctx, conn, version, string(body)); err != nil {
			return applied, fmt.Errorf("applying %s: %w", version, err)
		}
		applied = append(applied, version)
	}
	return applied, nil
}

func applyOne(ctx context.Context, conn *pgxpool.Conn, version, body string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("reading schema_migrations: %w", err)
	}
	defer rows.Close()

	done := map[string]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		done[version] = true
	}
	return done, rows.Err()
}

func available() ([]string, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("reading the embedded migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
