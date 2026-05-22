package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeySession ctxKey = iota + 1
	ctxKeyRequestID
)

// RequireSession ensures the X-Session-Id header is present and well-formed,
// and injects the session ID into the request context. Endpoints that don't
// require a session (e.g., GET /healthz) MUST NOT mount this middleware.
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-Session-Id")
		if raw == "" {
			WriteError(w, &APIError{
				Code:    CodeValidation,
				Message: "X-Session-Id header is required.",
			})
			return
		}
		if len(raw) < 1 || len(raw) > 128 {
			WriteError(w, &APIError{
				Code:    CodeValidation,
				Message: "X-Session-Id must be 1-128 characters.",
			})
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeySession, raw)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SessionFromContext returns the session id injected by RequireSession.
func SessionFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeySession).(string)
	return v, ok && v != ""
}

// RequestID generates a unique ID per request for tracing.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID set by RequestID middleware.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// Recover converts any panic in downstream handlers into a typed INTERNAL
// error so no stack traces leak to clients.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				slog.Error("panic in handler",
					"panic", p,
					"path", r.URL.Path,
					"method", r.Method,
					"request_id", RequestIDFromContext(r.Context()),
				)
				WriteError(w, &APIError{
					Code:    CodeInternal,
					Message: "An unexpected error occurred.",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder lets RequestLogger observe the response status.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// RequestLogger emits one structured log line per request.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		slog.Info("request",
			"request_id", RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// JSONContentType is a small helper to ensure JSON responses always carry
// the right content type even when handlers forget to set it.
func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
