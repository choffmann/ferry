ALTER TABLE bookings
    ADD COLUMN amount_cents   INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN claimed_at     TIMESTAMPTZ,
    ADD COLUMN settled_at     TIMESTAMPTZ,
    ADD COLUMN payment_ref    TEXT,
    ADD COLUMN failure_reason TEXT;

-- The worker only ever looks for open bookings, so the index does not have to
-- carry the ones it will never read again.
CREATE INDEX bookings_pending ON bookings (created_at)
    WHERE status = 'pending';
