//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/reservation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TestExpiry_TransitionsActiveToExpired verifies that a reservation whose
// expires_at has passed transitions to status='expired' via ExpireOne, and
// that the inventory view reflects the returned stock.
//
// The test sets expires_at into the past directly (rather than waiting 60s)
// so it stays fast and deterministic. The expirer worker uses the same
// ExpireOne function in production.
func TestExpiry_TransitionsActiveToExpired(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	res, err := reserve(saleID, session, 3, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed: err=%v status=%d body=%s", err, res.Status, string(res.BodyRaw))
	}

	// Force expires_at into the past. Move created_at back alongside it so
	// the schema CHECK (expires_at > created_at) is preserved.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx,
		`UPDATE reservations
		    SET created_at = now() - interval '10 seconds',
		        expires_at = now() - interval '1 second'
		  WHERE id = $1`,
		res.ReservationID,
	); err != nil {
		t.Fatalf("backdate timestamps: %v", err)
	}

	// Run the expirer's per-row transaction directly.
	reservationID, _ := uuid.Parse(res.ReservationID)
	if err := db.WithTx(ctx, pool, 2*time.Second, func(ctx context.Context, tx pgx.Tx) error {
		_, e := reservation.ExpireOne(ctx, tx, reservationID)
		return e
	}); err != nil {
		t.Fatalf("ExpireOne: %v", err)
	}

	// Verify status transitioned.
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1`, res.ReservationID,
	).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "expired" {
		t.Errorf("status = %q, want expired", status)
	}

	// Inventory restored.
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 0 || inv.AvailableToReserve != 10 {
		t.Errorf("inventory: reserved=%d available=%d, want 0/10",
			inv.CurrentlyReserved, inv.AvailableToReserve)
	}
}

// TestExpiry_NoOpOnTerminal verifies that ExpireOne against an already-
// released reservation is a clean no-op.
func TestExpiry_NoOpOnTerminal(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	res, err := reserve(saleID, session, 2, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed: err=%v status=%d", err, res.Status)
	}

	// Release first.
	resp, _ := releaseHTTP(t, res.ReservationID, session)
	if resp.StatusCode != 200 {
		t.Fatalf("release: status=%d", resp.StatusCode)
	}

	// Then attempt to expire (race-loser case): should be a no-op.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reservationID, _ := uuid.Parse(res.ReservationID)
	if err := db.WithTx(ctx, pool, 2*time.Second, func(ctx context.Context, tx pgx.Tx) error {
		transitioned, e := reservation.ExpireOne(ctx, tx, reservationID)
		if transitioned {
			t.Errorf("ExpireOne transitioned an already-released reservation")
		}
		return e
	}); err != nil {
		t.Fatalf("ExpireOne against terminal: %v", err)
	}

	// Status stays 'released' — release won the race.
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1`, res.ReservationID,
	).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "released" {
		t.Errorf("status drifted from released to %q after no-op expire", status)
	}
}
