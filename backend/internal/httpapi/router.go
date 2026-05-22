package httpapi

import (
	"net/http"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New builds the HTTP handler graph. Adds US2 inventory reads (list + single
// sale) and US4 reservation read/release on top of the US1 reservation create.
// CORS is permissive for local development; tighten for production.
func New(pool *pgxpool.Pool, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(Recover)
	r.Use(RequestLogger)
	r.Use(CORS)
	r.Use(JSONContentType)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Inventory (US2): list all sales with derived inventory, or read one.
	invH := &InventoryHandler{Pool: pool, Cfg: cfg}
	r.Get("/api/sales", invH.HandleList)
	r.Get("/api/sales/{sale_id}/inventory", invH.HandleGet)

	// Admin/seed: create a new sale.
	salesH := &SalesHandler{Pool: pool, Cfg: cfg}
	r.Post("/api/sales", salesH.HandleCreate)

	// Reservations (US1 + US4): create, get, release. All require a session.
	resH := &ReservationsHandler{Pool: pool, Cfg: cfg}
	r.Group(func(r chi.Router) {
		r.Use(RequireSession)
		r.Post("/api/sales/{sale_id}/reservations", resH.HandleCreate)
		r.Get("/api/reservations/{reservation_id}", resH.HandleGet)
		r.Delete("/api/reservations/{reservation_id}", resH.HandleDelete)
	})

	return r
}
