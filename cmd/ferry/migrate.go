package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/choffmann/ferry/internal/config"
	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/migrate"
	"github.com/choffmann/ferry/internal/store"
)

func runMigrate(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if err := parseNoFlags("migrate", args, stdout); err != nil {
		return err
	}

	pool, err := openDatabase(ctx, getenv)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := migrate.Apply(ctx, pool)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Fprintln(stdout, "schema is up to date")
		return nil
	}
	for _, version := range applied {
		fmt.Fprintf(stdout, "applied %s\n", version)
	}
	return nil
}

func runSeed(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if err := parseNoFlags("seed", args, stdout); err != nil {
		return err
	}

	pool, err := openDatabase(ctx, getenv)
	if err != nil {
		return err
	}
	defer pool.Close()

	zone, err := domain.LoadZone()
	if err != nil {
		return err
	}

	written, err := store.Seed(ctx, pool, time.Now().In(zone))
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %d connections and %d departures\n",
		written.Connections, written.Departures)
	return nil
}

func parseNoFlags(name string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stdout)
	return fs.Parse(args)
}

func openDatabase(ctx context.Context, getenv func(string) string) (*pgxpool.Pool, error) {
	url, err := config.DatabaseURL(getenv)
	if err != nil {
		return nil, err
	}
	return store.Connect(ctx, url)
}
