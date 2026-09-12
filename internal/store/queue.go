package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/choffmann/ferry/internal/domain"
)

// A worker that dies between the charge and the write leaves its claim behind.
// After this long the booking is handed out again rather than stuck forever.
const claimStaleAfter = time.Minute

func (s *PostgresStore) Claim(ctx context.Context, limit int) ([]domain.Booking, error) {
	now := s.now()
	// SKIP LOCKED is what keeps two workers off the same row. The claim is one
	// statement so that no transaction stays open across the HTTP call.
	rows, err := s.pool.Query(ctx, `
UPDATE bookings SET claimed_at = $1
WHERE id IN (
    SELECT id FROM bookings
    WHERE status = $2
      AND (claimed_at IS NULL OR claimed_at < $3)
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT $4
)
RETURNING id, departure_id, passengers, amount_cents, status, created_at`,
		now, domain.StatusPending, now.Add(-claimStaleAfter), limit)
	if err != nil {
		return nil, fmt.Errorf("claiming open bookings: %w", err)
	}
	defer rows.Close()

	var out []domain.Booking
	for rows.Next() {
		var b domain.Booking
		if err := rows.Scan(&b.ID, &b.DepartureID, &b.Passengers,
			&b.AmountCents, &b.Status, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("claiming open bookings: %w", err)
		}
		b.CreatedAt = b.CreatedAt.UTC()
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Settle(ctx context.Context, st domain.Settlement) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting the settlement transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// The status condition is what makes a second settlement impossible, and
	// with it a second release of the same seats.
	var departureID string
	var passengers int
	err = tx.QueryRow(ctx, `
UPDATE bookings
SET status = $2, payment_ref = NULLIF($3, ''), failure_reason = NULLIF($4, ''),
    settled_at = $5, claimed_at = NULL
WHERE id = $1 AND status = $6
RETURNING departure_id, passengers`,
		st.BookingID, st.Status, st.Reference, st.Reason, s.now(), domain.StatusPending).
		Scan(&departureID, &passengers)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: open booking %s", ErrNotFound, st.BookingID)
	}
	if err != nil {
		return fmt.Errorf("settling booking %s: %w", st.BookingID, err)
	}

	if st.Status == domain.StatusFailed {
		if _, err := tx.Exec(ctx,
			`UPDATE departures SET booked = booked - $2 WHERE id = $1`,
			departureID, passengers); err != nil {
			return fmt.Errorf("releasing the seats of %s: %w", st.BookingID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing the settlement of %s: %w", st.BookingID, err)
	}
	return nil
}

func (s *PostgresStore) Release(ctx context.Context, bookingID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE bookings SET claimed_at = NULL WHERE id = $1 AND status = $2`,
		bookingID, domain.StatusPending)
	if err != nil {
		return fmt.Errorf("releasing the claim on %s: %w", bookingID, err)
	}
	return nil
}
