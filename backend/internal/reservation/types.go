// Package reservation owns the consistency-critical business logic for the
// flash-sale system: creating reservations under contention, releasing them,
// expiring them, and applying idempotent retry semantics. Constitution
// principles I (consistency over availability) and II (pessimistic locking)
// govern every function in this package.
package reservation

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status mirrors the reservation_status enum in the database.
type Status string

const (
	StatusActive   Status = "active"
	StatusReleased Status = "released"
	StatusExpired  Status = "expired"
)

// Reservation is the in-memory representation of a row in the reservations
// table. JSON tags match contracts/api.md.
type Reservation struct {
	ID         uuid.UUID  `json:"id"`
	SaleID     uuid.UUID  `json:"sale_id"`
	SessionID  string     `json:"-"`
	Quantity   int        `json:"quantity"`
	Status     Status     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	ExpiredAt  *time.Time `json:"expired_at,omitempty"`
}

// CreateRequest is the validated form of the POST body for reservation create.
type CreateRequest struct {
	Quantity int `json:"quantity"`
}

// Validate enforces FR-006 (quantity must be > 0) before any database work.
func (req *CreateRequest) Validate() error {
	if req.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	return nil
}

// CreateParams is the resolved input passed to reservation.Create.
type CreateParams struct {
	ReservationID uuid.UUID
	SaleID        uuid.UUID
	SessionID     string
	Quantity      int
	TTL           time.Duration
}

// Typed errors. Handlers map these to API error codes.
var (
	ErrInvalidQuantity      = errors.New("reservation: quantity must be > 0")
	ErrSaleNotFound         = errors.New("reservation: sale not found")
	ErrReservationNotFound  = errors.New("reservation: reservation not found")
	ErrReservationTerminal  = errors.New("reservation: reservation is terminal")
)

// ErrInsufficientStock carries the current available count so the caller can
// surface it to the user (FR-008).
type ErrInsufficientStock struct {
	Available int
}

func (e *ErrInsufficientStock) Error() string { return "reservation: insufficient stock" }

// ErrIdempotencyKeyMismatch is returned by the idempotency helper when a
// retry arrives with a different payload than the original request that
// claimed the key.
type ErrIdempotencyKeyMismatch struct {
	OriginalHash  []byte
	SubmittedHash []byte
}

func (e *ErrIdempotencyKeyMismatch) Error() string { return "reservation: idempotency key mismatch" }
