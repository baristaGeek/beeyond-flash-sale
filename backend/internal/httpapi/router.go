package httpapi

import (
	"net/http"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New builds the HTTP handler graph for the MVP scope (US1: atomic reservation
// + idempotent retries). Subsequent user stories (US2..US6) add the inventory
// read endpoint, manual release, and the get-reservation endpoint; their
// routes are NOT registered here so any call to them returns 404 — making the
// scope of this branch obvious from the routing table alone.
func New(pool *pgxpool.Pool, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(Recover)
	r.Use(RequestLogger)
	r.Use(JSONContentType)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// US1: admin/seed endpoint — POST /api/sales.
	salesH := &SalesHandler{Pool: pool, Cfg: cfg}
	r.Post("/api/sales", salesH.HandleCreate)

	// US1: consistency-critical reservation create.
	resH := &ReservationsHandler{Pool: pool, Cfg: cfg}
	r.Group(func(r chi.Router) {
		r.Use(RequireSession)
		r.Post("/api/sales/{sale_id}/reservations", resH.HandleCreate)
	})

	return r
}
