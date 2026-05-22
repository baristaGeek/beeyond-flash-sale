//go:build integration

package integration

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

// TestConcurrentReservation_NoOversell drives 100 goroutines, each posting a
// 1-unit reservation against a sale with capacity 10. Verifies the headline
// constitution-driven guarantee: exactly 10 succeed and 90 receive
// INSUFFICIENT_STOCK, with no over-grant in the database.
func TestConcurrentReservation_NoOversell(t *testing.T) {
	mustSetup(t)

	const (
		capacity    = 10
		concurrency = 100
	)
	saleID := seedSale(t, capacity)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		successes   int
		conflicts   int
		other       int
	)
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		i := i
		go func() {
			defer wg.Done()
			sessionID := "session-" + uuid.NewString()
			_ = i
			res, err := reserve(saleID, sessionID, 1, "")
			if err != nil {
				t.Errorf("request error: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			switch {
			case res.Status == 201:
				successes++
			case res.ErrorCode == "INSUFFICIENT_STOCK":
				conflicts++
			default:
				other++
				t.Errorf("unexpected outcome: status=%d code=%s body=%s",
					res.Status, res.ErrorCode, string(res.BodyRaw))
			}
		}()
	}
	wg.Wait()

	if successes != capacity {
		t.Errorf("successes = %d, want %d", successes, capacity)
	}
	if conflicts != concurrency-capacity {
		t.Errorf("INSUFFICIENT_STOCK conflicts = %d, want %d", conflicts, concurrency-capacity)
	}
	if other != 0 {
		t.Errorf("unexpected outcomes: %d", other)
	}

	// Final DB state must match. This is the SC-001 / FR-007 invariant check.
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != capacity {
		t.Errorf("currently_reserved = %d, want %d", inv.CurrentlyReserved, capacity)
	}
	if inv.AvailableToReserve != 0 {
		t.Errorf("available_to_reserve = %d, want 0", inv.AvailableToReserve)
	}
	active, total := reservationCountInDB(t, saleID)
	if active != capacity || total != capacity {
		t.Errorf("active=%d total=%d, want both=%d", active, total, capacity)
	}
}
