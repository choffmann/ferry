package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/choffmann/ferry/internal/domain"
)

type PostgresStore struct {
	pool *pgxpool.Pool
	zone *time.Location
	now  func() time.Time
}

func NewPostgresStore(pool *pgxpool.Pool, zone *time.Location) *PostgresStore {
	return newPostgresStore(pool, zone, time.Now)
}

// zone is the wall clock the timetable is written in. Postgres hands timestamps
// back in UTC, so without converting them the local time of a departure would
// silently turn into an offset of +00:00 on the way out.
func newPostgresStore(pool *pgxpool.Pool, zone *time.Location, now func() time.Time) *PostgresStore {
	return &PostgresStore{pool: pool, zone: zone, now: now}
}

const departureColumns = "id, connection_id, departs_at, capacity, booked"

func (s *PostgresStore) Connections(ctx context.Context) ([]domain.Connection, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, from_code, from_name, to_code, to_name, duration_minutes
FROM connections
ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("reading connections: %w", err)
	}
	defer rows.Close()

	var out []domain.Connection
	for rows.Next() {
		var c domain.Connection
		if err := rows.Scan(&c.ID, &c.From.Code, &c.From.Name,
			&c.To.Code, &c.To.Name, &c.DurationMinutes); err != nil {
			return nil, fmt.Errorf("reading connections: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Departures(ctx context.Context, connectionID string, from time.Time) ([]domain.Departure, error) {
	// domain.BookingOpen read from the other side: a departure is on offer while
	// it lies further ahead than the boarding deadline.
	cutoff := s.now().Add(domain.BoardingClosesBefore)

	var since *time.Time
	if !from.IsZero() {
		since = &from
	}

	rows, err := s.pool.Query(ctx, `
SELECT `+departureColumns+`
FROM departures
WHERE connection_id = $1
  AND departs_at > $2
  AND ($3::timestamptz IS NULL OR departs_at >= $3)
ORDER BY departs_at`, connectionID, cutoff, since)
	if err != nil {
		return nil, fmt.Errorf("reading departures of %s: %w", connectionID, err)
	}
	defer rows.Close()

	var out []domain.Departure
	for rows.Next() {
		d, err := s.scanDeparture(rows)
		if err != nil {
			return nil, fmt.Errorf("reading departures of %s: %w", connectionID, err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Departure(ctx context.Context, id string) (domain.Departure, error) {
	d, err := s.scanDeparture(s.pool.QueryRow(ctx,
		`SELECT `+departureColumns+` FROM departures WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Departure{}, fmt.Errorf("%w: departure %s", ErrNotFound, id)
	}
	if err != nil {
		return domain.Departure{}, fmt.Errorf("reading departure %s: %w", id, err)
	}
	return d, nil
}

func (s *PostgresStore) Booking(ctx context.Context, id string) (domain.Booking, error) {
	var b domain.Booking
	err := s.pool.QueryRow(ctx, `
SELECT id, departure_id, passengers, status, created_at
FROM bookings
WHERE id = $1`, id).Scan(&b.ID, &b.DepartureID, &b.Passengers, &b.Status, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, fmt.Errorf("%w: booking %s", ErrNotFound, id)
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("reading booking %s: %w", id, err)
	}
	b.CreatedAt = b.CreatedAt.UTC()
	return b, nil
}

func (s *PostgresStore) Book(ctx context.Context, req domain.BookingRequest, opts domain.BookOptions) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("starting the booking transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// FOR UPDATE is what keeps two concurrent bookings from both passing the
	// capacity check on the same departure.
	d, err := s.scanDeparture(tx.QueryRow(ctx,
		`SELECT `+departureColumns+` FROM departures WHERE id = $1 FOR UPDATE`, req.DepartureID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, fmt.Errorf("%w: departure %s", ErrNotFound, req.DepartureID)
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("reading departure %s: %w", req.DepartureID, err)
	}

	if err := domain.CheckBookable(d, s.now(), req.Passengers, opts); err != nil {
		return domain.Booking{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE departures SET booked = booked + $2 WHERE id = $1`,
		d.ID, req.Passengers); err != nil {
		return domain.Booking{}, fmt.Errorf("updating the seat count of %s: %w", d.ID, err)
	}

	// The id comes from a sequence and Postgres rounds the timestamp to
	// microseconds, so both values are read back rather than guessed.
	b := domain.Booking{
		DepartureID: d.ID,
		Passengers:  req.Passengers,
		Status:      domain.StatusConfirmed,
	}
	if err := tx.QueryRow(ctx, `
INSERT INTO bookings (departure_id, passengers, status, created_at)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at`,
		d.ID, req.Passengers, domain.StatusConfirmed, s.now()).
		Scan(&b.ID, &b.CreatedAt); err != nil {
		return domain.Booking{}, fmt.Errorf("writing the booking: %w", err)
	}
	b.CreatedAt = b.CreatedAt.UTC()

	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("committing the booking: %w", err)
	}
	return b, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *PostgresStore) scanDeparture(row scanner) (domain.Departure, error) {
	var d domain.Departure
	if err := row.Scan(&d.ID, &d.ConnectionID, &d.DepartsAt, &d.Capacity, &d.Booked); err != nil {
		return domain.Departure{}, err
	}
	d.DepartsAt = d.DepartsAt.In(s.zone)
	return d, nil
}
