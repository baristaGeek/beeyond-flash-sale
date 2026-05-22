package httpapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/reservation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReservationsHandler serves the reservation create/release/get endpoints.
// MVP scope wires only HandleCreate; the others are added in US4 (T051-T052).
type ReservationsHandler struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

type createReservationRequest struct {
	Quantity int `json:"quantity"`
}

type reservationResponse struct {
	ID         uuid.UUID  `json:"id"`
	SaleID     uuid.UUID  `json:"sale_id"`
	Quantity   int        `json:"quantity"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	ExpiredAt  *time.Time `json:"expired_at,omitempty"`
}

// HandleCreate implements POST /api/sales/{sale_id}/reservations.
// This is the consistency-critical endpoint — see contracts/api.md § Endpoint 3
// and data-model.md § reservations.create.
func (h *ReservationsHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	saleIDStr := chi.URLParam(r, "sale_id")
	saleID, err := uuid.Parse(saleIDStr)
	if err != nil {
		WriteError(w, &APIError{Code: CodeValidation, Message: "Invalid sale id."})
		return
	}

	sessionID, ok := SessionFromContext(r.Context())
	if !ok {
		WriteError(w, &APIError{Code: CodeValidation, Message: "X-Session-Id header is required."})
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 4*1024))
	if err != nil {
		WriteError(w, &APIError{Code: CodeValidation, Message: "Failed to read request body."})
		return
	}

	var req createReservationRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		WriteError(w, &APIError{Code: CodeValidation, Message: "Malformed request body."})
		return
	}
	if req.Quantity <= 0 {
		WriteError(w, &APIError{Code: CodeInvalidQuantity, Message: "quantity must be > 0."})
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		WriteError(w, &APIError{
			Code:    CodeValidation,
			Message: "Idempotency-Key header is required.",
		})
		return
	}
	if len(idempotencyKey) > 128 {
		WriteError(w, &APIError{
			Code:    CodeValidation,
			Message: "Idempotency-Key must be 1-128 characters.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Build the request-hash up front so we can compare across replays.
	requestHash, hashErr := reservation.CanonicalRequestHash(saleID, rawBody)
	if hashErr != nil {
		WriteError(w, &APIError{Code: CodeInternal, Message: "Failed to hash request."})
		return
	}

	h.handleCreateWithIdempotency(ctx, w, saleID, sessionID, idempotencyKey, requestHash, req)
}

func (h *ReservationsHandler) handleCreateWithIdempotency(
	ctx context.Context,
	w http.ResponseWriter,
	saleID uuid.UUID,
	sessionID string,
	idempotencyKey string,
	requestHash []byte,
	req createReservationRequest,
) {
	var (
		responseStatus int
		responseBody   []byte
	)
	err := db.WithTx(ctx, h.Pool, h.Cfg.LockTimeout, func(ctx context.Context, tx pgx.Tx) error {
		cached, cachedResp, idemErr := reservation.WithIdempotency(
			ctx, tx, idempotencyKey, sessionID, saleID, requestHash,
			func(ctx context.Context) (int, []byte, *uuid.UUID, error) {
				r, createErr := reservation.Create(ctx, tx, reservation.CreateParams{
					SaleID:    saleID,
					SessionID: sessionID,
					Quantity:  req.Quantity,
					TTL:       h.Cfg.ReservationTTL,
				})
				if createErr != nil {
					return statusAndBodyForReservationError(createErr)
				}
				body, marshalErr := json.Marshal(toReservationResponse(r))
				if marshalErr != nil {
					return 0, nil, nil, marshalErr
				}
				return http.StatusCreated, body, &r.ID, nil
			},
		)
		if idemErr != nil {
			return idemErr
		}
		responseStatus = cachedResp.Status
		responseBody = cachedResp.Body
		_ = cached // currently unused; could be surfaced via header in future
		return nil
	})
	if err != nil {
		writeReservationError(w, err)
		return
	}
	if responseStatus == 0 || len(responseBody) == 0 {
		WriteError(w, &APIError{Code: CodeInternal, Message: "Empty idempotent response."})
		return
	}
	// Replay the cached body verbatim with its cached status code.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(responseStatus)
	_, _ = w.Write(responseBody)
}

// statusAndBodyForReservationError converts a reservation-layer error into the
// (status, body) tuple that the idempotency cache should store for replays.
// Rejections (INSUFFICIENT_STOCK) are also idempotent — they replay the same
// rejection on retries.
func statusAndBodyForReservationError(err error) (int, []byte, *uuid.UUID, error) {
	apiErr := reservationErrorToAPI(err)
	if apiErr == nil {
		// Treat unrecognized errors as transient: propagate the error so the
		// outer WithTx rolls back, discards the idempotency placeholder, and
		// the client can retry.
		return 0, nil, nil, err
	}
	body, marshalErr := json.Marshal(errorEnvelope{Error: apiErr})
	if marshalErr != nil {
		return 0, nil, nil, marshalErr
	}
	return apiErr.HTTPStatus(), body, nil, nil
}

// reservationErrorToAPI maps internal reservation errors to API errors.
// Returns nil for errors that should propagate as transient (e.g., DB-level
// lock_timeout — that path is handled by MapDBError after rollback).
func reservationErrorToAPI(err error) *APIError {
	if err == nil {
		return nil
	}
	if errors.Is(err, reservation.ErrInvalidQuantity) {
		return &APIError{Code: CodeInvalidQuantity, Message: "quantity must be > 0."}
	}
	if errors.Is(err, reservation.ErrSaleNotFound) {
		return &APIError{Code: CodeSaleNotFound, Message: "Sale not found."}
	}
	if errors.Is(err, reservation.ErrReservationNotFound) {
		return &APIError{Code: CodeReservationNotFound, Message: "Reservation not found."}
	}
	var insuff *reservation.ErrInsufficientStock
	if errors.As(err, &insuff) {
		return &APIError{
			Code:    CodeInsufficientStock,
			Message: "Not enough stock is currently available.",
			Details: map[string]any{"available_to_reserve": insuff.Available},
		}
	}
	var mismatch *reservation.ErrIdempotencyKeyMismatch
	if errors.As(err, &mismatch) {
		return &APIError{
			Code:    CodeIdempotencyKeyMismatch,
			Message: "This idempotency key was used previously with a different payload.",
			Details: map[string]any{
				"original_request_hash":  hex.EncodeToString(bytes.Clone(mismatch.OriginalHash)),
				"submitted_request_hash": hex.EncodeToString(bytes.Clone(mismatch.SubmittedHash)),
			},
		}
	}
	return nil
}

// writeReservationError performs final error mapping at the handler boundary.
func writeReservationError(w http.ResponseWriter, err error) {
	if apiErr := reservationErrorToAPI(err); apiErr != nil {
		WriteError(w, apiErr)
		return
	}
	if dbErr := MapDBError(err); dbErr != nil {
		WriteError(w, dbErr)
		return
	}
	WriteError(w, &APIError{Code: CodeInternal, Message: "Reservation request failed."})
}

func toReservationResponse(r *reservation.Reservation) reservationResponse {
	return reservationResponse{
		ID:         r.ID,
		SaleID:     r.SaleID,
		Quantity:   r.Quantity,
		Status:     string(r.Status),
		CreatedAt:  r.CreatedAt,
		ExpiresAt:  r.ExpiresAt,
		ReleasedAt: r.ReleasedAt,
		ExpiredAt:  r.ExpiredAt,
	}
}
