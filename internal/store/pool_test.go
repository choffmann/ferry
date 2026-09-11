package store

import (
	"context"
	"testing"

	"github.com/choffmann/ferry/internal/pgtest"
)

func TestConnectReturnsAUsablePool(t *testing.T) {
	pool, err := Connect(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	var one int
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&one); err != nil {
		t.Errorf("querying through the pool: %v", err)
	}
}

// Waiting for the database belongs in the description of the topology, not in
// the application, so an unreachable database is a startup error.
func TestConnectFailsWhenNothingListens(t *testing.T) {
	_, err := Connect(context.Background(),
		"postgres://ferry:ferry@127.0.0.1:1/ferry?sslmode=disable&connect_timeout=2")
	if err == nil {
		t.Error("Connect to an address nothing listens on returned no error")
	}
}
