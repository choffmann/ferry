// Package pgtest hands an integration test a database it may write to. Every
// caller gets its own schema, so the cases never see each other's rows without
// needing a database each.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const URLVar = "TEST_DATABASE_URL"

// URL skips the test when TEST_DATABASE_URL is unset. Call it from the test
// itself when the pool is built inside a subtest, otherwise the skip would land
// on the parent.
func URL(t *testing.T) string {
	t.Helper()
	raw := os.Getenv(URLVar)
	if raw == "" {
		t.Skipf("%s is not set, skipping the integration test", URLVar)
	}
	return raw
}

// DSN creates an empty schema and returns a connection string that resolves
// unqualified table names inside it. The schema is dropped when the test ends.
func DSN(t *testing.T) string {
	t.Helper()

	raw := URL(t)
	schema := "t_" + randomSuffix(t)
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatalf("connecting to %s: %v", URLVar, err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("creating schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("dropping schema %s: %v", schema, err)
		}
		admin.Close()
	})

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %s: %v", URLVar, err)
	}
	q := parsed.Query()
	// No space: url encoding would turn it into a plus and the backend would
	// then look for a parameter called "+search_path".
	q.Set("options", "-csearch_path="+schema)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// Pool is DSN with a pool already open on it. The schema is empty: callers that
// need tables run the migrations themselves.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), DSN(t))
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
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
