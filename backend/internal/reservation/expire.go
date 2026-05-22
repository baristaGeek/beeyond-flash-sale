package reservation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ExpireOne transitions a single reservation row from 'active' to 'expired'
// if it is still active. No-op (returns nil, nil) if release won the race or
// another expirer already processed it.
//
// MUST be called inside a transaction opened by the expirer. The caller is
// responsible for SELECT ... FOR UPDATE on the row before invoking ExpireOne.
func ExpireOne(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID) (transitioned bool, err error) {
	var status Status
	err = tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1 FOR UPDATE`,
		reservationID,
	).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("expire: lock: %w", err)
	}
	if status != StatusActive {
		return false, nil
	}

	tag, err := tx.Exec(ctx,
		`UPDATE reservations
		    SET status = 'expired', expired_at = now()
		  WHERE id = $1 AND status = 'active'`,
		reservationID,
	)
	if err != nil {
		return false, fmt.Errorf("expire: update: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SelectExpiringCandidates returns up to `limit` reservation IDs whose
// expires_at has passed. Read-only, no lock.
func SelectExpiringCandidates(ctx context.Context, tx pgx.Tx, limit int) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx,
		`SELECT id
		   FROM reservations
		  WHERE status = 'active' AND expires_at <= now()
		  ORDER BY expires_at
		  LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("expire: select candidates: %w", err)
	}
	defer rows.Close()

	out := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("expire: scan: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
