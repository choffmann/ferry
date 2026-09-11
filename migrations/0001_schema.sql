CREATE TABLE connections (
    id               TEXT PRIMARY KEY,
    from_code        TEXT    NOT NULL,
    from_name        TEXT    NOT NULL,
    to_code          TEXT    NOT NULL,
    to_name          TEXT    NOT NULL,
    duration_minutes INTEGER NOT NULL
);

CREATE TABLE departures (
    id            TEXT PRIMARY KEY,
    connection_id TEXT        NOT NULL REFERENCES connections (id),
    departs_at    TIMESTAMPTZ NOT NULL,
    capacity      INTEGER     NOT NULL,
    booked        INTEGER     NOT NULL DEFAULT 0 CHECK (booked >= 0)
);

CREATE INDEX departures_by_connection ON departures (connection_id, departs_at);

CREATE SEQUENCE booking_ids;

-- The id format bk-000001 is part of the API and predates the database, so the
-- sequence is formatted rather than exposed as a number.
CREATE TABLE bookings (
    id           TEXT PRIMARY KEY
                 DEFAULT 'bk-' || lpad(nextval('booking_ids')::text, 6, '0'),
    departure_id TEXT        NOT NULL REFERENCES departures (id),
    passengers   INTEGER     NOT NULL,
    status       TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL
);
