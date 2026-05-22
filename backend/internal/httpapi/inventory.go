package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InventoryHandler serves the read-only inventory endpoints. These reads are
// best-effort (no transaction lock) per the design: dashboard freshness budget
// is ≤1s, far looser than the consistency budget of reservation writes.
type InventoryHandler struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

type inventoryRow struct {
	SaleID             uuid.UUID `json:"sale_id"`
	Name               string    `json:"name"`
	TotalCapacity      int       `json:"total_capacity"`
	CurrentlyReserved  int       `json:"currently_reserved"`
	AvailableToReserve int       `json:"available_to_reserve"`
}

type listResponse struct {
	Sales []inventoryRow `json:"sales"`
}

// HandleList implements GET /api/sales. Returns every sale with its derived
// inventory in one round-trip so the frontend doesn't need N+1 polls. Sorted
// by created_at ascending so the order is stable across polls.
func (h *InventoryHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.Pool.Query(ctx,
		`SELECT s.id, s.name, s.total_capacity,
		        COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0)::int AS reserved,
		        (s.total_capacity
		           - COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0))::int AS available
		   FROM sales s
		   LEFT JOIN reservations r ON r.sale_id = s.id
		  GROUP BY s.id
		  ORDER BY s.created_at`,
	)
	if err != nil {
		WriteError(w, MapDBError(err))
		return
	}
	defer rows.Close()

	out := listResponse{Sales: []inventoryRow{}}
	for rows.Next() {
		var row inventoryRow
		if err := rows.Scan(&row.SaleID, &row.Name, &row.TotalCapacity, &row.CurrentlyReserved, &row.AvailableToReserve); err != nil {
			WriteError(w, &APIError{Code: CodeInternal, Message: "Failed to scan inventory row."})
			return
		}
		out.Sales = append(out.Sales, row)
	}
	if err := rows.Err(); err != nil {
		WriteError(w, MapDBError(err))
		return
	}

	WriteJSON(w, http.StatusOK, out)
}

// HandleGet implements GET /api/sales/{sale_id}/inventory. Used for deep links
// and single-sale polling. Returns 404 SALE_NOT_FOUND if the sale does not exist.
func (h *InventoryHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	saleIDStr := chi.URLParam(r, "sale_id")
	saleID, err := uuid.Parse(saleIDStr)
	if err != nil {
		WriteError(w, &APIError{Code: CodeValidation, Message: "Invalid sale id."})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var row inventoryRow
	err = h.Pool.QueryRow(ctx,
		`SELECT s.id, s.name, s.total_capacity,
		        COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0)::int,
		        (s.total_capacity
		           - COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0))::int
		   FROM sales s
		   LEFT JOIN reservations r ON r.sale_id = s.id
		  WHERE s.id = $1
		  GROUP BY s.id`,
		saleID,
	).Scan(&row.SaleID, &row.Name, &row.TotalCapacity, &row.CurrentlyReserved, &row.AvailableToReserve)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, &APIError{Code: CodeSaleNotFound, Message: "Sale not found."})
			return
		}
		WriteError(w, MapDBError(err))
		return
	}
	WriteJSON(w, http.StatusOK, row)
}
