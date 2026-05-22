// Package expirer runs the background TTL worker that transitions active
// reservations to 'expired' once their expires_at has passed. The worker
// polls every Cfg.ExpirerPollInterval (default 500ms), selects a small batch
// of expired-but-active reservations, and processes each in its own short
// transaction so the per-row lock window stays tight and concurrent
// reservation creates are not blocked.
package expirer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/reservation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker is the background TTL expirer.
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

const expireBatchSize = 100

func (w *Worker) run(ctx context.Context) {
	defer close(w.done)
	t := time.NewTicker(w.cfg.ExpirerPollInterval)
	defer t.Stop()
	slog.Info("expirer started", "interval", w.cfg.ExpirerPollInterval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("expirer stopped")
			return
		case <-t.C:
			if err := w.tick(ctx); err != nil {
				slog.Warn("expirer tick error", "err", err)
			}
		}
	}
}

// tick finds a batch of expired-but-active reservations and processes each in
// its own transaction. Errors on individual reservations are logged and do
// not abort the batch — the next tick will try them again.
func (w *Worker) tick(ctx context.Context) error {
	candidates, err := w.selectCandidates(ctx)
	if err != nil {
		return err
	}
	for _, id := range candidates {
		if err := w.expireOne(ctx, id); err != nil {
			slog.Warn("expire one", "reservation_id", id, "err", err)
		}
	}
	return nil
}

func (w *Worker) selectCandidates(ctx context.Context) ([]uuid.UUID, error) {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx,
		`SELECT id
		   FROM reservations
		  WHERE status = 'active' AND expires_at <= now()
		  ORDER BY expires_at
		  LIMIT $1`,
		expireBatchSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]uuid.UUID, 0, expireBatchSize)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (w *Worker) expireOne(ctx context.Context, id uuid.UUID) error {
	return db.WithTx(ctx, w.pool, w.cfg.LockTimeout, func(ctx context.Context, tx pgx.Tx) error {
		_, err := reservation.ExpireOne(ctx, tx, id)
		return err
	})
}
