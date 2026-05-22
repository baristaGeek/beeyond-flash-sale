// Package db exposes a Postgres connection pool and a boundary-level
// transaction helper that satisfies Constitution Principle I
// ("Transactions at the boundary"). Every mutating HTTP handler MUST
// open its transaction via WithTx so the SET LOCAL lock_timeout is
// applied consistently and rollback-on-error is guaranteed.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New constructs a pgx connection pool sized for a small/medium load.
// The pool is opened eagerly so DB unreachability fails fast at startup.
func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	// Bounded pool. The flash-sale workload is short transactions; 20 conns
	// is plenty for hundreds of concurrent reservations at the API tier.
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// TxFunc is the unit of work that runs inside WithTx.
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// WithTx opens a transaction, applies the supplied per-transaction settings
// (always SET LOCAL lock_timeout to the supplied value), runs fn, and commits
// or rolls back deterministically.
//
// fn MUST NOT call tx.Commit / tx.Rollback itself.
func WithTx(ctx context.Context, pool *pgxpool.Pool, lockTimeout time.Duration, fn TxFunc) (err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.Background())
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(context.Background())
		}
	}()

	// SET LOCAL lock_timeout = '...ms' — bounded waits, fail-closed on timeout.
	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", lockTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("db: set lock_timeout: %w", err)
	}

	if err = fn(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// IsLockTimeout reports whether err is a Postgres lock_timeout (SQLSTATE 55P03).
func IsLockTimeout(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "55P03"
	}
	return false
}
