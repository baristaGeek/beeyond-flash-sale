package reservation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Release transitions an active reservation to 'released' and returns its units
// to the parent sale's available pool. It MUST be called inside a transaction
// opened by db.WithTx; the caller manages the lock_timeout.
//
// Locking strategy: SELECT ... FOR UPDATE on the reservation row only. The
// release does NOT touch the parent sales row; the inventory view recomputes
// Available from the active reservations after this transaction commits.
//
// Idempotent behavior:
//   - If the reservation does not exist OR belongs to a different session, returns
//     ErrReservationNotFound. (Treating foreign-session as not-found avoids
//     leaking existence; per contracts/api.md it surfaces as RESERVATION_NOT_FOUND.)
//   - If the reservation is already terminal (released or expired), returns
//     ErrReservationTerminal with the current row so the caller can echo
//     status back to the client (FR-014, idempotent release).
//   - On the happy path, sets status='released', released_at=now(), returns
//     the updated row.
func Release(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID, sessionID string) (*Reservation, error) {
	r := &Reservation{}
	err := tx.QueryRow(ctx,
		`SELECT id, sale_id, session_id, quantity, created_at, expires_at,
		        released_at, expired_at, status
		   FROM reservations
		  WHERE id = $1
		    FOR UPDATE`,
		reservationID,
	).Scan(&r.ID, &r.SaleID, &r.SessionID, &r.Quantity, &r.CreatedAt, &r.ExpiresAt,
		&r.ReleasedAt, &r.ExpiredAt, &r.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReservationNotFound
		}
		return nil, fmt.Errorf("release: lock reservation: %w", err)
	}

	if r.SessionID != sessionID {
		return nil, ErrReservationNotFound
	}

	if r.Status != StatusActive {
		// Already terminal — return current row so caller can render it.
		return r, ErrReservationTerminal
	}

	err = tx.QueryRow(ctx,
		`UPDATE reservations
		    SET status = 'released', released_at = now()
		  WHERE id = $1 AND status = 'active'
		  RETURNING released_at, status`,
		reservationID,
	).Scan(&r.ReleasedAt, &r.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Lost the race to the TTL expirer between SELECT FOR UPDATE
			// and UPDATE. Re-read to surface the terminal state cleanly.
			return nil, ErrReservationTerminal
		}
		return nil, fmt.Errorf("release: update: %w", err)
	}
	return r, nil
}

// Get reads a reservation by id, scoped to the supplied session. Returns
// ErrReservationNotFound if missing or owned by a different session.
// Used by the frontend's countdown-timer resync via GET /api/reservations/{id}.
func Get(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID, sessionID string) (*Reservation, error) {
	r := &Reservation{}
	err := tx.QueryRow(ctx,
		`SELECT id, sale_id, session_id, quantity, created_at, expires_at,
		        released_at, expired_at, status
		   FROM reservations
		  WHERE id = $1`,
		reservationID,
	).Scan(&r.ID, &r.SaleID, &r.SessionID, &r.Quantity, &r.CreatedAt, &r.ExpiresAt,
		&r.ReleasedAt, &r.ExpiredAt, &r.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReservationNotFound
		}
		return nil, fmt.Errorf("get: %w", err)
	}
	if r.SessionID != sessionID {
		return nil, ErrReservationNotFound
	}
	return r, nil
}
