//go:build integration

package integration

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

// TestIdempotency_ConcurrentSameKey verifies FR-027: concurrent reservations
// carrying the same idempotency key serialize at the database such that
// exactly one performs the stock decrement and all others observe the cached
// outcome.
func TestIdempotency_ConcurrentSameKey(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 100)
	session := "session-" + uuid.NewString()
	key := "idem-" + uuid.NewString()

	const concurrency = 10
	results := make([]reserveResult, concurrency)
	errs := make([]error, concurrency)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		i := i
		go func() {
			defer wg.Done()
			res, err := reserve(saleID, session, 4, key)
			results[i] = res
			errs[i] = err
		}()
	}
	wg.Wait()

	var reservationID string
	for i, res := range results {
		if errs[i] != nil {
			t.Fatalf("request %d error: %v", i, errs[i])
		}
		if res.Status != 201 {
			t.Fatalf("request %d: status=%d code=%s body=%s",
				i, res.Status, res.ErrorCode, string(res.BodyRaw))
		}
		if res.ReservationID == "" {
			t.Fatalf("request %d: empty reservation id; body=%s", i, string(res.BodyRaw))
		}
		if reservationID == "" {
			reservationID = res.ReservationID
		} else if res.ReservationID != reservationID {
			t.Errorf("request %d: reservation id %s differs from canonical %s",
				i, res.ReservationID, reservationID)
		}
	}

	// Stock decremented exactly once (4 units, not 40).
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 4 {
		t.Errorf("currently_reserved = %d, want 4", inv.CurrentlyReserved)
	}
	active, total := reservationCountInDB(t, saleID)
	if active != 1 || total != 1 {
		t.Errorf("expected exactly 1 reservation, got active=%d total=%d", active, total)
	}
}
