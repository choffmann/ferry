// Package pgtest hands an integration test a database it may write to. Every
// caller gets its own schema, so the cases never see each other's rows without
// needing a database each.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const URLVar = "TEST_DATABASE_URL"

// Pool skips the test when TEST_DATABASE_URL is unset. The schema it creates is
// empty: callers that need tables run the migrations themselves.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv(URLVar)
	if url == "" {
		t.Skipf("%s is not set, skipping the integration test", URLVar)
	}

	ctx := context.Background()
	schema := "t_" + randomSuffix(t)

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting to %s: %v", URLVar, err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("creating schema %s: %v", schema, err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parsing %s: %v", URLVar, err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to schema %s: %v", schema, err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("dropping schema %s: %v", schema, err)
		}
		admin.Close()
	})
	return pool
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("random schema name: %v", err)
	}
	return hex.EncodeToString(b[:])
}
