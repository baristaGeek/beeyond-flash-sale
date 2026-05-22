//go:build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
)

// TestIdempotency_SameKeySamePayload verifies FR-024:
// two requests with the same Idempotency-Key and identical payload return
// the same reservation ID and decrement stock exactly once.
func TestIdempotency_SameKeySamePayload(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	key := "idem-" + uuid.NewString()

	first, err := reserve(saleID, session, 3, key)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Status != 201 || first.ReservationID == "" {
		t.Fatalf("first request failed: status=%d code=%s body=%s",
			first.Status, first.ErrorCode, string(first.BodyRaw))
	}

	second, err := reserve(saleID, session, 3, key)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Status != 201 {
		t.Fatalf("second request: status=%d body=%s", second.Status, string(second.BodyRaw))
	}
	if second.ReservationID != first.ReservationID {
		t.Errorf("reservation IDs differ: first=%s second=%s", first.ReservationID, second.ReservationID)
	}

	// Stock decremented exactly once (3 units, not 6).
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 3 {
		t.Errorf("currently_reserved = %d, want 3", inv.CurrentlyReserved)
	}
	active, total := reservationCountInDB(t, saleID)
	if active != 1 || total != 1 {
		t.Errorf("expected exactly 1 active/total reservation, got active=%d total=%d", active, total)
	}
}

// TestIdempotency_SameKeyDifferentPayload verifies FR-025:
// two requests with the same Idempotency-Key but different payloads — the
// second must be rejected with IDEMPOTENCY_KEY_MISMATCH and must not touch
// stock.
func TestIdempotency_SameKeyDifferentPayload(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	key := "idem-" + uuid.NewString()

	first, err := reserve(saleID, session, 3, key)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Status != 201 {
		t.Fatalf("first: status=%d body=%s", first.Status, string(first.BodyRaw))
	}

	second, err := reserve(saleID, session, 5, key) // different quantity
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.ErrorCode != "IDEMPOTENCY_KEY_MISMATCH" {
		t.Errorf("expected IDEMPOTENCY_KEY_MISMATCH, got status=%d code=%s body=%s",
			second.Status, second.ErrorCode, string(second.BodyRaw))
	}
	if second.Status != 422 {
		t.Errorf("expected HTTP 422 for IDEMPOTENCY_KEY_MISMATCH, got %d", second.Status)
	}

	// Stock unchanged from the first request (3 units, not 8).
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 3 {
		t.Errorf("currently_reserved = %d, want 3", inv.CurrentlyReserved)
	}
}

// TestIdempotency_RejectedReplay verifies that replays of rejected requests
// (e.g., INSUFFICIENT_STOCK) are also idempotent — the same rejection is
// returned, stock unchanged.
func TestIdempotency_RejectedReplay(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 2)
	session := "session-" + uuid.NewString()
	key := "idem-" + uuid.NewString()

	// First request asks for more than capacity → INSUFFICIENT_STOCK.
	first, err := reserve(saleID, session, 5, key)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.ErrorCode != "INSUFFICIENT_STOCK" {
		t.Fatalf("expected INSUFFICIENT_STOCK, got %s", first.ErrorCode)
	}
	if first.Available != 2 {
		t.Errorf("first.available = %d, want 2", first.Available)
	}

	// Replay should return the same rejection.
	second, err := reserve(saleID, session, 5, key)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.ErrorCode != "INSUFFICIENT_STOCK" {
		t.Errorf("replay returned %s, want INSUFFICIENT_STOCK", second.ErrorCode)
	}
	if second.Status != 409 {
		t.Errorf("replay status = %d, want 409", second.Status)
	}

	// No reservations should have been created.
	active, total := reservationCountInDB(t, saleID)
	if active != 0 || total != 0 {
		t.Errorf("expected 0 reservations, got active=%d total=%d", active, total)
	}
}
