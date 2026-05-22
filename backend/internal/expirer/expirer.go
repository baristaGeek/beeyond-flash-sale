// Package expirer runs the background TTL worker that transitions active
// reservations to 'expired' once their expires_at has passed.
//
// This file is an MVP-scope stub: in branch 003-backend-mvp the worker is a
// no-op so cmd/server compiles and runs. User Story 3 (T042-T044) replaces
// the body of Worker.tick with the real polling logic against the
// reservations table.
package expirer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker is the background TTL expirer. Stub for MVP; real implementation lands in US3.
type Worker struct {
	pool *pgxpool.Pool
	cfg  *config.Config

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewWorker constructs a Worker bound to the given pool and config.
func NewWorker(pool *pgxpool.Pool, cfg *config.Config) *Worker {
	return &Worker{pool: pool, cfg: cfg, done: make(chan struct{})}
}

// Start kicks off the polling loop. Safe to call exactly once.
func (w *Worker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)
	go w.run(ctx)
}

// Stop signals the worker to exit and waits for it to finish.
func (w *Worker) Stop() {
	w.once.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
		<-w.done
	})
}

func (w *Worker) run(ctx context.Context) {
	defer close(w.done)
	t := time.NewTicker(w.cfg.ExpirerPollInterval)
	defer t.Stop()
	slog.Info("expirer started (MVP stub: no-op)", "interval", w.cfg.ExpirerPollInterval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("expirer stopped")
			return
		case <-t.C:
			// MVP stub: real polling logic lands in US3 (T042-T044).
		}
	}
}
