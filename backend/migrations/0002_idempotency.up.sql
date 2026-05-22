-- 0002_idempotency.up.sql
-- Backs the Idempotency-Key header on POST /api/sales/{id}/reservations.
-- See specs/001-flash-sale-reservation/data-model.md § idempotency_records.

BEGIN;

CREATE TABLE idempotency_records (
    idempotency_key text        NOT NULL,
    session_id      text        NOT NULL,
    sale_id         uuid        NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    request_hash    bytea       NOT NULL,
    response_status smallint    NOT NULL,
    response_body   jsonb       NOT NULL,
    reservation_id  uuid        REFERENCES reservations(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (idempotency_key, session_id),
    CONSTRAINT idempotency_key_length CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    CONSTRAINT idempotency_status_range CHECK (response_status BETWEEN 0 AND 599)
);

-- Used by the GC sweep that prunes records older than the retention window.
CREATE INDEX idempotency_records_gc_idx
    ON idempotency_records (created_at);

COMMIT;
