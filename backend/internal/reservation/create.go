package reservation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Create is the consistency-critical core of the flash-sale system.
// It MUST be called inside a transaction opened by db.WithTx so the
// lock_timeout is bounded; this function does NOT manage the transaction
// lifecycle itself.
//
// Locking strategy (Constitution Principle II):
//   1. SELECT id, total_capacity FROM sales WHERE id=$1 FOR UPDATE — serialize
//      every concurrent reservation-create against the same sale.
//   2. Compute SUM(quantity) of active reservations inside the lock.
//   3. If total_capacity - reserved >= requested, INSERT the new row;
//      otherwise return ErrInsufficientStock with the current available count.
//
// No partial grants. No optimistic CAS. No retries that mask race conditions.
func Create(ctx context.Context, tx pgx.Tx, p CreateParams) (*Reservation, error) {
	if p.Quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("reservation: empty session id")
	}
	if p.TTL <= 0 {
		return nil, fmt.Errorf("reservation: ttl must be > 0")
	}

	// 1. Lock the sale row. SALE_NOT_FOUND if absent.
	var totalCapacity int
	err := tx.QueryRow(ctx,
		`SELECT total_capacity FROM sales WHERE id = $1 FOR UPDATE`,
		p.SaleID,
	).Scan(&totalCapacity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSaleNotFound
		}
		return nil, fmt.Errorf("reservation create: lock sale: %w", err)
	}

	// 2. Sum currently-active reservations inside the lock.
	var reserved int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(quantity), 0)
		   FROM reservations
		  WHERE sale_id = $1 AND status = 'active'`,
		p.SaleID,
	).Scan(&reserved)
	if err != nil {
		return nil, fmt.Errorf("reservation create: sum active: %w", err)
	}

	available := totalCapacity - reserved
	if available < p.Quantity {
		return nil, &ErrInsufficientStock{Available: available}
	}

	// 3. Insert the new reservation.
	if p.ReservationID == uuid.Nil {
		p.ReservationID = uuid.New()
	}
	ttlSeconds := int(p.TTL.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO reservations
		   (id, sale_id, session_id, quantity, expires_at, status)
		 VALUES
		   ($1, $2, $3, $4, now() + make_interval(secs => $5), 'active')
		 RETURNING id, sale_id, session_id, quantity, created_at, expires_at, status`,
		p.ReservationID, p.SaleID, p.SessionID, p.Quantity, ttlSeconds,
	)
	r := &Reservation{}
	if err := row.Scan(&r.ID, &r.SaleID, &r.SessionID, &r.Quantity, &r.CreatedAt, &r.ExpiresAt, &r.Status); err != nil {
		return nil, fmt.Errorf("reservation create: insert: %w", err)
	}
	return r, nil
}

