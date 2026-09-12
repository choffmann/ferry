// Package worker settles the bookings the API has taken. It is the second
// workload of the ferry binary and runs as ferry worker.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/payments"
	"github.com/choffmann/ferry/internal/store"
)

const (
	interval   = time.Second
	batchSize  = 10
	askTimeout = 5 * time.Second
)

// Charger is the payment service as the worker needs it. payments.Client is the
// implementation the binary passes in.
type Charger interface {
	Charge(ctx context.Context, req payments.ChargeRequest) (payments.ChargeResponse, error)
}

type Worker struct {
	queue    store.Queue
	payments Charger
	logger   *slog.Logger
}

func New(q store.Queue, p Charger, l *slog.Logger) *Worker {
	return &Worker{queue: q, payments: p, logger: l}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := w.Tick(ctx); err != nil && ctx.Err() == nil {
				w.logger.Error("settling the open bookings failed", "error", err)
			}
		}
	}
}

// Tick settles one batch and reports how many bookings it wrote back. It is the
// unit the tests drive, so no case has to wait for a ticker.
func (w *Worker) Tick(ctx context.Context) (int, error) {
	open, err := w.queue.Claim(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	settled := 0

	for _, b := range open {
		wg.Add(1)
		go func(b domain.Booking) {
			defer wg.Done()
			if w.settle(ctx, b) {
				mu.Lock()
				settled++
				mu.Unlock()
			}
		}(b)
	}
	wg.Wait()
	return settled, nil
}

func (w *Worker) settle(ctx context.Context, b domain.Booking) bool {
	askCtx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()

	res, err := w.payments.Charge(askCtx, payments.ChargeRequest{
		BookingID: b.ID, AmountCents: b.AmountCents,
	})
	if err != nil {
		// A silent payment service says nothing about the payment, so the
		// booking goes back into the queue instead of being marked failed.
		w.logger.Warn("the payment service did not answer, the booking stays open",
			"booking_id", b.ID, "error", err)
		if err := w.queue.Release(ctx, b.ID); err != nil {
			w.logger.Error("the claim could not be released",
				"booking_id", b.ID, "error", err)
		}
		return false
	}

	s := domain.Settlement{BookingID: b.ID}
	if res.Status == payments.Approved {
		s.Status, s.Reference = domain.StatusConfirmed, res.Reference
	} else {
		s.Status, s.Reason = domain.StatusFailed, res.Reason
	}

	if err := w.queue.Settle(ctx, s); err != nil {
		w.logger.Error("the outcome could not be written back",
			"booking_id", b.ID, "status", s.Status, "error", err)
		return false
	}
	w.logger.Info("booking settled",
		"booking_id", b.ID, "status", s.Status, "amount_cents", b.AmountCents)
	return true
}
