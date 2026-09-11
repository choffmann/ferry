package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/choffmann/ferry/internal/pgtest"
)

func TestRunMigrateAppliesTheSchemaOnce(t *testing.T) {
	dsn := pgtest.DSN(t)
	ctx := context.Background()

	var first bytes.Buffer
	if err := runMigrate(ctx, nil, env(map[string]string{"DATABASE_URL": dsn}), &first); err != nil {
		t.Fatalf("first runMigrate: %v", err)
	}
	if !strings.Contains(first.String(), "0001_schema.sql") {
		t.Errorf("the first run did not report the migration:\n%s", first.String())
	}

	var second bytes.Buffer
	if err := runMigrate(ctx, nil, env(map[string]string{"DATABASE_URL": dsn}), &second); err != nil {
		t.Fatalf("second runMigrate: %v", err)
	}
	if strings.Contains(second.String(), "0001_schema.sql") {
		t.Errorf("the second run applied the migration again:\n%s", second.String())
	}
}

// The migration service in a compose file gets the database and nothing else.
func TestRunMigrateDoesNotNeedTheAdminToken(t *testing.T) {
	dsn := pgtest.DSN(t)
	err := runMigrate(context.Background(), nil,
		env(map[string]string{"DATABASE_URL": dsn, "ADMIN_TOKEN": ""}), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("runMigrate without ADMIN_TOKEN: %v", err)
	}
}

func TestRunMigrateStopsWithoutTheDatabaseURL(t *testing.T) {
	err := runMigrate(context.Background(), nil,
		env(map[string]string{"DATABASE_URL": ""}), &bytes.Buffer{})
	if err == nil {
		t.Fatal("runMigrate without DATABASE_URL returned no error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error %q does not name DATABASE_URL", err)
	}
}

func TestRunSeedWritesTheTimetableAndIsRepeatable(t *testing.T) {
	dsn := pgtest.DSN(t)
	ctx := context.Background()
	e := env(map[string]string{"DATABASE_URL": dsn})

	if err := runMigrate(ctx, nil, e, &bytes.Buffer{}); err != nil {
		t.Fatalf("runMigrate: %v", err)
	}

	var first bytes.Buffer
	if err := runSeed(ctx, nil, e, &first); err != nil {
		t.Fatalf("first runSeed: %v", err)
	}
	if !strings.Contains(first.String(), "departures") {
		t.Errorf("the first run did not report what it wrote:\n%s", first.String())
	}

	var second bytes.Buffer
	if err := runSeed(ctx, nil, e, &second); err != nil {
		t.Fatalf("second runSeed: %v", err)
	}
	if !strings.Contains(second.String(), "0 departures") {
		t.Errorf("the second run wrote rows again:\n%s", second.String())
	}
}

func TestRunSeedReportsAMissingSchema(t *testing.T) {
	dsn := pgtest.DSN(t)
	err := runSeed(context.Background(), nil,
		env(map[string]string{"DATABASE_URL": dsn}), &bytes.Buffer{})
	if err == nil {
		t.Fatal("runSeed against an empty schema returned no error")
	}
}
