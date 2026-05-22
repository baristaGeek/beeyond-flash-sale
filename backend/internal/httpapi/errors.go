// Package httpapi implements the chi router, middleware, and HTTP handlers
// for the flash-sale backend. All errors leave handlers as typed APIErrors
// so the frontend can switch on the code rather than parsing English.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
)

// ErrorCode is a stable machine-readable identifier for every failure mode
// that crosses the API boundary. See specs/001-flash-sale-reservation/contracts/api.md.
type ErrorCode string

const (
	CodeInvalidQuantity        ErrorCode = "INVALID_QUANTITY"
	CodeSaleNotFound           ErrorCode = "SALE_NOT_FOUND"
	CodeReservationNotFound    ErrorCode = "RESERVATION_NOT_FOUND"
	CodeReservationTerminal    ErrorCode = "RESERVATION_TERMINAL"
	CodeInsufficientStock      ErrorCode = "INSUFFICIENT_STOCK"
	CodeIdempotencyKeyMismatch ErrorCode = "IDEMPOTENCY_KEY_MISMATCH"
	CodeLockTimeout            ErrorCode = "LOCK_TIMEOUT"
	CodeValidation             ErrorCode = "VALIDATION"
	CodeInternal               ErrorCode = "INTERNAL"
)

// APIError is the wire-level error envelope produced by every handler.
type APIError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error implements error so APIError can be returned from internal helpers.
func (e *APIError) Error() string { return string(e.Code) + ": " + e.Message }

// HTTPStatus returns the HTTP status code paired with the error code.
// Idempotent terminal-replay is intentionally a 200 — it is not a true error.
func (e *APIError) HTTPStatus() int {
	switch e.Code {
	case CodeReservationTerminal:
		return http.StatusOK
	case CodeInvalidQuantity, CodeValidation:
		return http.StatusBadRequest
	case CodeSaleNotFound, CodeReservationNotFound:
		return http.StatusNotFound
	case CodeInsufficientStock:
		return http.StatusConflict
	case CodeIdempotencyKeyMismatch:
		return http.StatusUnprocessableEntity
	case CodeLockTimeout:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// errorEnvelope matches the JSON shape in contracts/api.md § Error envelope.
type errorEnvelope struct {
	Error *APIError `json:"error"`
}

// WriteError serializes err as the API error envelope. err MUST be an *APIError;
// any other error is wrapped as INTERNAL so we never leak stack traces.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		apiErr = &APIError{
			Code:    CodeInternal,
			Message: "An unexpected error occurred.",
		}
		slog.Error("untyped error reached WriteError", "err", err)
	}
	WriteJSON(w, apiErr.HTTPStatus(), errorEnvelope{Error: apiErr})
}

// WriteJSON serializes v as JSON with the given status. Failures are logged
// and produce no further response (the client will see a truncated body).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}

// MapDBError converts a low-level Postgres or pgx error into the appropriate
// API error code. Lock timeout (Principle I: fail closed) is recognized here.
func MapDBError(err error) *APIError {
	if err == nil {
		return nil
	}
	if db.IsLockTimeout(err) {
		return &APIError{
			Code:    CodeLockTimeout,
			Message: "The server is busy; please retry the request.",
		}
	}
	return &APIError{
		Code:    CodeInternal,
		Message: "Database error.",
	}
}
