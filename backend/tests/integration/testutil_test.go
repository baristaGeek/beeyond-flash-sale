//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	setupOnce sync.Once
	pool      *pgxpool.Pool
	server    *httptest.Server
	cfg       *config.Config
)

func mustSetup(t *testing.T) {
	t.Helper()
	setupOnce.Do(func() {
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			databaseURL = "postgres://flashsale:flashsale@localhost:5432/flashsale?sslmode=disable"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Resolve migrations directory relative to this test file.
		_, thisFile, _, _ := runtime.Caller(0)
		migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
		absMigrations, err := filepath.Abs(migrationsDir)
		if err != nil {
			t.Fatalf("resolve migrations dir: %v", err)
		}

		// Apply forward migrations idempotently.
		if err := db.Migrate(databaseURL, absMigrations, db.Up); err != nil {
			t.Fatalf("migrate up: %v", err)
		}

		p, err := db.New(ctx, databaseURL)
		if err != nil {
			t.Fatalf("connect db: %v", err)
		}
		pool = p

		cfg = &config.Config{
			DatabaseURL:          databaseURL,
			HTTPAddr:             ":0",
			LockTimeout:          5 * time.Second,
			ExpirerPollInterval:  500 * time.Millisecond,
			IdempotencyRetention: 24 * time.Hour,
			ReservationTTL:       60 * time.Second,
		}
		server = httptest.NewServer(httpapi.New(pool, cfg))
	})

	// Truncate state between tests so each starts from a known baseline.
	truncate(t)
}

func truncate(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, `TRUNCATE TABLE idempotency_records, reservations, sales RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedSale(t *testing.T, capacity int) uuid.UUID {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"name":           fmt.Sprintf("test-sale-%d", time.Now().UnixNano()),
		"total_capacity": capacity,
	})
	resp, err := http.Post(server.URL+"/api/sales", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("seed sale: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("seed sale: status %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode seed response: %v", err)
	}
	return out.ID
}

type reserveResult struct {
	Status        int
	BodyRaw       []byte
	ReservationID string
	ErrorCode     string
	Available     int
}

func reserve(saleID uuid.UUID, sessionID string, quantity int, idempotencyKey string) (reserveResult, error) {
	body, _ := json.Marshal(map[string]any{"quantity": quantity})
	req, _ := http.NewRequest(http.MethodPost,
		server.URL+"/api/sales/"+saleID.String()+"/reservations",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Id", sessionID)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return reserveResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	res := reserveResult{Status: resp.StatusCode, BodyRaw: raw}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var ok struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &ok)
		res.ReservationID = ok.ID
		return res, nil
	}
	var env struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		res.ErrorCode = env.Error.Code
		if v, ok := env.Error.Details["available_to_reserve"]; ok {
			res.Available = toInt(v)
		}
	}
	return res, nil
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

// inventoryFromDB queries the live inventory view directly from the database.
// Used by integration tests to verify final state without depending on the
// inventory HTTP endpoint (which lands in US2).
type inventoryRow struct {
	TotalCapacity      int
	CurrentlyReserved  int
	AvailableToReserve int
}

func inventoryFromDB(t *testing.T, saleID uuid.UUID) inventoryRow {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var inv inventoryRow
	err := pool.QueryRow(ctx,
		`SELECT s.total_capacity,
		        COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0)::int,
		        s.total_capacity - COALESCE(SUM(r.quantity) FILTER (WHERE r.status = 'active'), 0)::int
		   FROM sales s
		   LEFT JOIN reservations r ON r.sale_id = s.id
		  WHERE s.id = $1
		  GROUP BY s.id`,
		saleID,
	).Scan(&inv.TotalCapacity, &inv.CurrentlyReserved, &inv.AvailableToReserve)
	if err != nil {
		t.Fatalf("inventoryFromDB: %v", err)
	}
	return inv
}

func reservationCountInDB(t *testing.T, saleID uuid.UUID) (active int, total int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.QueryRow(ctx,
		`SELECT
		   COUNT(*) FILTER (WHERE status = 'active'),
		   COUNT(*)
		 FROM reservations
		 WHERE sale_id = $1`,
		saleID,
	).Scan(&active, &total); err != nil {
		t.Fatalf("reservationCountInDB: %v", err)
	}
	return
}
