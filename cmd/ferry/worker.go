package main

import (
	"context"
	"io"
	"time"

	"github.com/choffmann/ferry/internal/config"
	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/payments"
	"github.com/choffmann/ferry/internal/store"
	"github.com/choffmann/ferry/internal/worker"
)

const paymentsTimeout = 5 * time.Second

func runWorker(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if err := parseNoFlags("worker", args, stdout); err != nil {
		return err
	}

	cfg, err := config.Worker(getenv)
	if err != nil {
		return err
	}

	zone, err := domain.LoadZone()
	if err != nil {
		return err
	}

	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	logger := obs.NewLogger(stdout)
	logger.Info("ferry worker is starting", "payments", cfg.PaymentsURL)

	w := worker.New(
		store.NewPostgresStore(pool, zone),
		payments.NewClient(cfg.PaymentsURL, paymentsTimeout),
		logger,
	)
	err = w.Run(ctx)
	logger.Info("ferry worker is stopping")
	return err
}
