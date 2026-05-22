// Command seed inserts a deterministic demo sale into the database so the
// frontend always has something to point at during development. Idempotent:
// re-running is a no-op via INSERT ... ON CONFLICT DO NOTHING.
//
// Usage: go run ./cmd/seed
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/config"
	"github.com/baristaGeek/beeyond-flash-sale/backend/internal/db"
	"github.com/google/uuid"
)

// DemoSaleID is the deterministic identifier used by both the seed command
// and the frontend's default landing page. Treat it as a public constant
// of the development build.
const DemoSaleID = "00000001-0000-4000-8000-000000000000"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	name := flag.String("name", "Demo Flash Sale", "Human-readable name for the seeded sale.")
	capacity := flag.Int("capacity", 100, "Total capacity of the seeded sale.")
	flag.Parse()

	if *capacity <= 0 {
		slog.Error("capacity must be > 0", "capacity", *capacity)
		os.Exit(1)
	}

	saleID, err := uuid.Parse(DemoSaleID)
	if err != nil {
		slog.Error("invalid DemoSaleID constant", "err", err)
		os.Exit(1)
	}

	cfg := config.MustLoad()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Idempotent insert. If the sale already exists we leave it untouched —
	// this lets `make seed` be safe to run repeatedly without resetting
	// inventory mid-demo.
	tag, err := pool.Exec(ctx,
		`INSERT INTO sales (id, name, total_capacity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		saleID, *name, *capacity,
	)
	if err != nil {
		slog.Error("insert seed sale", "err", err)
		os.Exit(1)
	}

	if tag.RowsAffected() == 0 {
		fmt.Printf("Demo sale already present: id=%s\n", saleID)
	} else {
		fmt.Printf("Seeded demo sale: id=%s name=%q capacity=%d\n", saleID, *name, *capacity)
	}
	fmt.Printf("\nFrontend URL (default): http://localhost:5173/\n")
	fmt.Printf("Explicit URL:           http://localhost:5173/?sale=%s\n", saleID)
	fmt.Printf("API inventory URL:      http://localhost:8080/api/sales/%s/inventory\n", saleID)
}
