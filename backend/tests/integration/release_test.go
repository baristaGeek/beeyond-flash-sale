//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func releaseHTTP(t *testing.T, reservationID string, sessionID string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete,
		server.URL+"/api/reservations/"+reservationID, nil)
	req.Header.Set("X-Session-Id", sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

// TestRelease_HappyPath verifies that releasing an active reservation returns
// 200, marks the reservation released in the DB, and frees the stock back to
// the available pool exactly once.
func TestRelease_HappyPath(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	res, err := reserve(saleID, session, 3, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed: err=%v status=%d body=%s", err, res.Status, string(res.BodyRaw))
	}

	resp, body := releaseHTTP(t, res.ReservationID, session)
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
	var got struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		Code       string `json:"code"`
		ReleasedAt string `json:"released_at"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != "released" {
		t.Errorf("status = %q, want released (body: %s)", got.Status, string(body))
	}
	if got.Code == "RESERVATION_TERMINAL" {
		t.Errorf("happy-path release should not carry RESERVATION_TERMINAL code")
	}

	// Inventory restored.
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 0 || inv.AvailableToReserve != 10 {
		t.Errorf("post-release inventory: reserved=%d available=%d, want 0/10",
			inv.CurrentlyReserved, inv.AvailableToReserve)
	}
}

// TestRelease_IdempotentReleased verifies that releasing an already-released
// reservation returns 200 with code=RESERVATION_TERMINAL and does not touch
// the inventory a second time (FR-014).
func TestRelease_IdempotentReleased(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	res, err := reserve(saleID, session, 2, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed: err=%v status=%d", err, res.Status)
	}

	// First release: happy path.
	resp1, _ := releaseHTTP(t, res.ReservationID, session)
	if resp1.StatusCode != 200 {
		t.Fatalf("first release status=%d", resp1.StatusCode)
	}

	// Second release: idempotent terminal response.
	resp2, body2 := releaseHTTP(t, res.ReservationID, session)
	if resp2.StatusCode != 200 {
		t.Fatalf("second release status=%d body=%s", resp2.StatusCode, string(body2))
	}
	var terminal struct {
		Status string `json:"status"`
		Code   string `json:"code"`
	}
	_ = json.Unmarshal(body2, &terminal)
	if terminal.Code != "RESERVATION_TERMINAL" {
		t.Errorf("expected code=RESERVATION_TERMINAL, got %q (body: %s)", terminal.Code, string(body2))
	}
	if terminal.Status != "released" {
		t.Errorf("expected status=released, got %q", terminal.Status)
	}

	// Inventory unchanged from the first release.
	inv := inventoryFromDB(t, saleID)
	if inv.CurrentlyReserved != 0 || inv.AvailableToReserve != 10 {
		t.Errorf("inventory drifted after idempotent release: reserved=%d available=%d",
			inv.CurrentlyReserved, inv.AvailableToReserve)
	}
}

// TestRelease_UnknownReservation verifies 404 RESERVATION_NOT_FOUND for an id
// that does not exist or belongs to another session.
func TestRelease_UnknownReservation(t *testing.T) {
	mustSetup(t)
	session := "session-" + uuid.NewString()

	resp, body := releaseHTTP(t, uuid.NewString(), session)
	if resp.StatusCode != 404 {
		t.Fatalf("unknown id status=%d body=%s", resp.StatusCode, string(body))
	}

	// Foreign session: should also be 404 (do not leak existence).
	saleID := seedSale(t, 10)
	other := "session-" + uuid.NewString()
	res, err := reserve(saleID, other, 1, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed: err=%v status=%d", err, res.Status)
	}
	resp2, body2 := releaseHTTP(t, res.ReservationID, session)
	if resp2.StatusCode != 404 {
		t.Errorf("foreign-session release status=%d body=%s, want 404", resp2.StatusCode, string(body2))
	}
}

// TestGetReservation_HappyPath verifies the read endpoint returns a
// reservation owned by the calling session.
func TestGetReservation_HappyPath(t *testing.T) {
	mustSetup(t)
	saleID := seedSale(t, 10)
	session := "session-" + uuid.NewString()
	res, err := reserve(saleID, session, 4, "idem-"+uuid.NewString())
	if err != nil || res.Status != 201 {
		t.Fatalf("seed: err=%v status=%d", err, res.Status)
	}

	req, _ := http.NewRequest(http.MethodGet,
		server.URL+"/api/reservations/"+res.ReservationID, nil)
	req.Header.Set("X-Session-Id", session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var got struct {
		ID       string `json:"id"`
		Quantity int    `json:"quantity"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != res.ReservationID || got.Quantity != 4 || got.Status != "active" {
		t.Errorf("got %+v, want id=%s quantity=4 status=active", got, res.ReservationID)
	}
}
