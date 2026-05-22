//go:build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
)

// TestReservation_RequiresIdempotencyKey verifies FR-023: a reservation-create
// request without the Idempotency-Key header is rejected with a typed
// VALIDATION error and no stock is decremented.
func TestReservation_RequiresIdempotencyKey(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()

	res, err := reserve(saleID, session, 3, "") // empty Idempotency-Key
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if res.Status != 400 {
		t.Errorf("expected HTTP 400, got %d (body: %s)", res.Status, string(res.BodyRaw))
	}
	if res.ErrorCode != "VALIDATION" {
		t.Errorf("expected VALIDATION error code, got %q (body: %s)", res.ErrorCode, string(res.BodyRaw))
	}

	// Stock untouched.
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 0 {
		t.Errorf("currently_reserved = %d, want 0 (no stock should be decremented without Idempotency-Key)", inv.CurrentlyReserved)
	}
	active, total := reservationCountInDB(t, saleID)
	if active != 0 || total != 0 {
		t.Errorf("expected no reservations created, got active=%d total=%d", active, total)
	}
}
