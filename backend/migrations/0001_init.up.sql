-- 0001_init.up.sql
-- Initial schema for the flash-sale reservation system.
-- Constitution principles I-III are enforced at the DB layer:
--   * Two 3NF tables; available stock is derived, never stored.
--   * Every invariant declared as CHECK / NOT NULL / FK.

BEGIN;

CREATE TYPE reservation_status AS ENUM ('active', 'released', 'expired');

CREATE TABLE sales (
    id              uuid        PRIMARY KEY,
    name            text        NOT NULL,
    total_capacity  integer     NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sales_name_length CHECK (length(name) BETWEEN 1 AND 200),
    CONSTRAINT sales_capacity_positive CHECK (total_capacity > 0)
);

CREATE TABLE reservations (
    id           uuid                PRIMARY KEY,
    sale_id      uuid                NOT NULL REFERENCES sales(id) ON DELETE RESTRICT,
    session_id   text                NOT NULL,
    quantity     integer             NOT NULL,
    created_at   timestamptz         NOT NULL DEFAULT now(),
    expires_at   timestamptz         NOT NULL,
    released_at  timestamptz,
    expired_at   timestamptz,
    status       reservation_status  NOT NULL,
    CONSTRAINT reservations_session_length CHECK (length(session_id) BETWEEN 1 AND 128),
    CONSTRAINT reservations_quantity_positive CHECK (quantity > 0),
    CONSTRAINT reservations_expires_after_created CHECK (expires_at > created_at),
    CONSTRAINT reservations_status_timestamps CHECK (
        (status = 'active'   AND released_at IS NULL AND expired_at IS NULL) OR
        (status = 'released' AND released_at IS NOT NULL AND expired_at IS NULL) OR
        (status = 'expired'  AND released_at IS NULL AND expired_at IS NOT NULL)
    )
);

-- Partial index used by the inventory query (sum active by sale).
CREATE INDEX reservations_sale_active_idx
    ON reservations (sale_id)
    WHERE status = 'active';

-- Partial index used by the TTL expirer worker.
CREATE INDEX reservations_active_expiring_idx
    ON reservations (expires_at)
    WHERE status = 'active';

-- Lookup by session.
CREATE INDEX reservations_session_idx
    ON reservations (session_id);

COMMIT;
