package reservation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CachedResponse is the verbatim HTTP response stored in idempotency_records.
type CachedResponse struct {
	Status        int             `json:"-"`
	Body          json.RawMessage `json:"-"`
	ReservationID *uuid.UUID      `json:"-"`
}

// IdempotentRunner is the unit of work that runs when this request is the
// first to claim the (key, session) slot. Implementations MUST return the
// HTTP status and the JSON body bytes that should be cached and returned to
// the original caller and to every future replay that matches the hash.
//
// reservationID, if non-nil, will be linked into the idempotency record so
// the foreign key trail survives even when the reservation is later
// tombstoned. Pass nil for rejected requests.
type IdempotentRunner func(ctx context.Context) (status int, body []byte, reservationID *uuid.UUID, err error)

// WithIdempotency executes the "claim-or-observe" pattern against
// idempotency_records. See specs/001-flash-sale-reservation/data-model.md
// § reservations.create with Idempotency-Key.
//
// Behavior:
//   - First request with this (key, session): runs `run`, persists the response,
//     returns (cached=false, run's status/body).
//   - Replay with matching hash: returns (cached=true, the original status/body).
//   - Replay with different hash: returns ErrIdempotencyKeyMismatch.
//
// MUST be called inside a transaction. The caller is responsible for
// BEGIN/COMMIT via db.WithTx.
func WithIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	key string,
	sessionID string,
	saleID uuid.UUID,
	requestHash []byte,
	run IdempotentRunner,
) (cached bool, resp CachedResponse, err error) {
	// Attempt to claim the (key, session) slot. INSERT ... ON CONFLICT DO NOTHING
	// is the canonical PostgreSQL race-winner / race-loser primitive.
	emptyBody := []byte("{}")
	var claimedKey string
	insertErr := tx.QueryRow(ctx,
		`INSERT INTO idempotency_records
		   (idempotency_key, session_id, sale_id, request_hash,
		    response_status, response_body, reservation_id)
		 VALUES ($1, $2, $3, $4, 0, $5::jsonb, NULL)
		 ON CONFLICT (idempotency_key, session_id) DO NOTHING
		 RETURNING idempotency_key`,
		key, sessionID, saleID, requestHash, string(emptyBody),
	).Scan(&claimedKey)

	if insertErr != nil && !errors.Is(insertErr, pgx.ErrNoRows) {
		return false, CachedResponse{}, fmt.Errorf("idempotency: claim: %w", insertErr)
	}

	// Branch A: we won the race. Run the work and persist the response.
	if !errors.Is(insertErr, pgx.ErrNoRows) {
		status, body, reservationID, runErr := run(ctx)
		if runErr != nil {
			// On error, the failing transaction will roll back which also
			// discards the placeholder idempotency record — so a future
			// retry with the same key behaves as a fresh request, which is
			// the right semantics for transient failures.
			return false, CachedResponse{}, runErr
		}
		_, err = tx.Exec(ctx,
			`UPDATE idempotency_records
			    SET response_status = $1,
			        response_body   = $2::jsonb,
			        reservation_id  = $3
			  WHERE idempotency_key = $4 AND session_id = $5`,
			status, string(body), reservationID, key, sessionID,
		)
		if err != nil {
			return false, CachedResponse{}, fmt.Errorf("idempotency: persist response: %w", err)
		}
		return false, CachedResponse{Status: status, Body: body, ReservationID: reservationID}, nil
	}

	// Branch B: the slot was already claimed. Lock the existing row (this will
	// block on the winner's transaction until they commit/rollback), then
	// compare hashes.
	var (
		existingHash  []byte
		existingStat  int16
		existingBody  []byte
		existingResID *uuid.UUID
	)
	err = tx.QueryRow(ctx,
		`SELECT request_hash, response_status, response_body, reservation_id
		   FROM idempotency_records
		  WHERE idempotency_key = $1 AND session_id = $2
		    FOR UPDATE`,
		key, sessionID,
	).Scan(&existingHash, &existingStat, &existingBody, &existingResID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Winner rolled back; their placeholder is gone. Surface as a
			// transient error so the client retries (which will re-enter
			// branch A on the next request).
			return false, CachedResponse{}, fmt.Errorf("idempotency: claim row vanished")
		}
		return false, CachedResponse{}, fmt.Errorf("idempotency: observe: %w", err)
	}
	if !bytes.Equal(existingHash, requestHash) {
		return false, CachedResponse{}, &ErrIdempotencyKeyMismatch{
			OriginalHash:  existingHash,
			SubmittedHash: requestHash,
		}
	}
	return true, CachedResponse{
		Status:        int(existingStat),
		Body:          existingBody,
		ReservationID: existingResID,
	}, nil
}
