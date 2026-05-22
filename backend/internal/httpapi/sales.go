package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SalesHandler serves the admin/seed sale endpoints.
type SalesHandler struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

type createSaleRequest struct {
	Name          string `json:"name"`
	TotalCapacity int    `json:"total_capacity"`
}

type saleResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	TotalCapacity int       `json:"total_capacity"`
	CreatedAt     time.Time `json:"created_at"`
}

// HandleCreate implements POST /api/sales. Seed/admin endpoint; not exposed
// in the customer UI but used by the load harness and by the dashboard demo.
func (h *SalesHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var body createSaleRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		WriteError(w, &APIError{
			Code:    CodeValidation,
			Message: "Malformed request body.",
		})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || len(body.Name) > 200 {
		WriteError(w, &APIError{
			Code:    CodeValidation,
			Message: "name must be 1-200 characters.",
		})
		return
	}
	if body.TotalCapacity <= 0 {
		WriteError(w, &APIError{
			Code:    CodeValidation,
			Message: "total_capacity must be > 0.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := saleResponse{
		ID:            uuid.New(),
		Name:          body.Name,
		TotalCapacity: body.TotalCapacity,
	}

	err := db.WithTx(ctx, h.Pool, h.Cfg.LockTimeout, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO sales (id, name, total_capacity)
			 VALUES ($1, $2, $3)
			 RETURNING created_at`,
			resp.ID, resp.Name, resp.TotalCapacity,
		).Scan(&resp.CreatedAt)
	})
	if err != nil {
		if apiErr := MapDBError(err); apiErr != nil && !errors.Is(err, context.Canceled) {
			WriteError(w, apiErr)
			return
		}
		WriteError(w, &APIError{Code: CodeInternal, Message: "Failed to create sale."})
		return
	}

	WriteJSON(w, http.StatusCreated, resp)
}
