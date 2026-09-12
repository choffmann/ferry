// Package worker settles the bookings the API has taken. It is the second
// workload of the ferry binary and runs as ferry worker.
package worker

import (
	"context"
	"errors"
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
		return w.releaseAndKeepOpen(ctx, b.ID)
	}

	s := domain.Settlement{BookingID: b.ID}
	switch res.Status {
	case payments.Approved:
		s.Status, s.Reference = domain.StatusConfirmed, res.Reference
	case payments.Declined:
		s.Status, s.Reason = domain.StatusFailed, res.Reason
	default:
		// Charger is an exported interface; a status that is neither approved
		// nor declined is treated like a silent outage, not a decline, so an
		// implementation other than payments.Client cannot fail bookings by
		// returning something unexpected.
		w.logger.Warn("the payment service answered with an unrecognised status, the booking stays open",
			"booking_id", b.ID, "status", res.Status)
		return w.releaseAndKeepOpen(ctx, b.ID)
	}

	if err := w.queue.Settle(ctx, s); err != nil {
		// A stale claim can be picked up by two workers; the loser's Settle
		// matches no row and is a benign race, not a write failure.
		if errors.Is(err, store.ErrNotFound) {
			w.logger.Info("the booking was already settled by another worker",
				"booking_id", b.ID, "error", err)
			return false
		}
		w.logger.Error("the outcome could not be written back",
			"booking_id", b.ID, "status", s.Status, "error", err)
		return false
	}
	w.logger.Info("booking settled",
		"booking_id", b.ID, "status", s.Status, "amount_cents", b.AmountCents)
	return true
}

func (w *Worker) releaseAndKeepOpen(ctx context.Context, bookingID string) bool {
	if err := w.queue.Release(ctx, bookingID); err != nil {
		w.logger.Error("the claim could not be released",
			"booking_id", bookingID, "error", err)
	}
	return false
}
