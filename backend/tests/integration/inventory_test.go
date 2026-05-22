//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestInventory_ListEmpty verifies GET /api/sales returns an empty list when
// no sales exist.
func TestInventory_ListEmpty(t *testing.T) {
	mustSetup(t)

	resp, err := http.Get(server.URL + "/api/sales")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Sales []any `json:"sales"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Sales) != 0 {
		t.Errorf("expected 0 sales, got %d", len(body.Sales))
	}
}

// TestInventory_ListMany verifies GET /api/sales returns each sale with its
// computed inventory and that the invariant total = reserved + available
// holds for every row.
func TestInventory_ListMany(t *testing.T) {
	mustSetup(t)
	saleA := seedSale(t, 20)
	saleB := seedSale(t, 10)

	// Reserve 3 against A.
	res, err := reserve(saleA, "session-A", 3, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed reservation: err=%v status=%d body=%s", err, res.Status, string(res.BodyRaw))
	}

	resp, err := http.Get(server.URL + "/api/sales")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var body struct {
		Sales []struct {
			SaleID             string `json:"sale_id"`
			Name               string `json:"name"`
			TotalCapacity      int    `json:"total_capacity"`
			CurrentlyReserved  int    `json:"currently_reserved"`
			AvailableToReserve int    `json:"available_to_reserve"`
		} `json:"sales"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode (%s): %v", string(raw), err)
	}
	if len(body.Sales) != 2 {
		t.Fatalf("expected 2 sales, got %d (body: %s)", len(body.Sales), string(raw))
	}

	var sumA, sumB bool
	for _, s := range body.Sales {
		if s.TotalCapacity != s.CurrentlyReserved+s.AvailableToReserve {
			t.Errorf("invariant violation for %s: total=%d reserved=%d available=%d",
				s.Name, s.TotalCapacity, s.CurrentlyReserved, s.AvailableToReserve)
		}
		switch s.SaleID {
		case saleA.String():
			if s.CurrentlyReserved != 3 || s.AvailableToReserve != 17 {
				t.Errorf("sale A: reserved=%d available=%d, want 3/17", s.CurrentlyReserved, s.AvailableToReserve)
			}
			sumA = true
		case saleB.String():
			if s.CurrentlyReserved != 0 || s.AvailableToReserve != 10 {
				t.Errorf("sale B: reserved=%d available=%d, want 0/10", s.CurrentlyReserved, s.AvailableToReserve)
			}
			sumB = true
		}
	}
	if !sumA || !sumB {
		t.Errorf("missing sales in response: A=%v B=%v", sumA, sumB)
	}
}

// TestInventory_GetSingle verifies GET /api/sales/{id}/inventory returns the
// expected counts and 404 on unknown ids.
func TestInventory_GetSingle(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)

	resp, err := http.Get(server.URL + "/api/sales/" + saleID.String() + "/inventory")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var inv struct {
		TotalCapacity      int `json:"total_capacity"`
		CurrentlyReserved  int `json:"currently_reserved"`
		AvailableToReserve int `json:"available_to_reserve"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if inv.TotalCapacity != 10 || inv.CurrentlyReserved != 0 || inv.AvailableToReserve != 10 {
		t.Errorf("got total=%d reserved=%d available=%d, want 10/0/10",
			inv.TotalCapacity, inv.CurrentlyReserved, inv.AvailableToReserve)
	}

	// Unknown id -> 404 SALE_NOT_FOUND.
	resp2, err := http.Get(server.URL + "/api/sales/" + uuid.NewString() + "/inventory")
	if err != nil {
		t.Fatalf("get unknown: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Errorf("unknown id status = %d, want 404", resp2.StatusCode)
	}
}
