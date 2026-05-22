// Command seed inserts the demo product catalog into the database so the
// frontend always has something to display. Idempotent: re-running is a no-op
// via INSERT ... ON CONFLICT DO NOTHING. The deterministic UUIDs let the
// frontend deep-link to a specific product.
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

// DemoProducts is the seeded catalog. The UUIDs are deterministic so the
// frontend and any test fixtures can reference them directly.
type DemoProduct struct {
	ID       string
	Name     string
	Capacity int
}

var DemoProducts = []DemoProduct{
	{ID: "00000001-0000-4000-8000-000000000001", Name: "Vintage Camera", Capacity: 20},
	{ID: "00000001-0000-4000-8000-000000000002", Name: "Mechanical Watch", Capacity: 10},
	{ID: "00000001-0000-4000-8000-000000000003", Name: "Acoustic Guitar", Capacity: 16},
	{ID: "00000001-0000-4000-8000-000000000004", Name: "Smart Flask", Capacity: 20},
	{ID: "00000001-0000-4000-8000-000000000005", Name: "Running Shoes", Capacity: 12},
	{ID: "00000001-0000-4000-8000-000000000006", Name: "Gaming Mouse", Capacity: 15},
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	wipe := flag.Bool("wipe", false, "Wipe ALL sales and reservations before seeding (destructive).")
	flag.Parse()

	cfg := config.MustLoad()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if *wipe {
		fmt.Println("Wiping idempotency_records, reservations, sales...")
		if _, err := pool.Exec(ctx,
			`TRUNCATE TABLE idempotency_records, reservations, sales RESTART IDENTITY CASCADE`,
		); err != nil {
			slog.Error("wipe", "err", err)
			os.Exit(1)
		}
	}

	inserted := 0
	for _, p := range DemoProducts {
		id, err := uuid.Parse(p.ID)
		if err != nil {
			slog.Error("invalid product id", "id", p.ID, "err", err)
			os.Exit(1)
		}
		tag, err := pool.Exec(ctx,
			`INSERT INTO sales (id, name, total_capacity)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (id) DO NOTHING`,
			id, p.Name, p.Capacity,
		)
		if err != nil {
			slog.Error("insert", "name", p.Name, "err", err)
			os.Exit(1)
		}
		if tag.RowsAffected() == 1 {
			inserted++
			fmt.Printf("  seeded: %s (capacity %d) id=%s\n", p.Name, p.Capacity, p.ID)
		} else {
			fmt.Printf("  exists: %s id=%s\n", p.Name, p.ID)
		}
	}

	fmt.Printf("\n%d new product(s) inserted; %d already present.\n", inserted, len(DemoProducts)-inserted)
	fmt.Printf("\nFrontend URL: http://localhost:5173/\n")
	fmt.Printf("API list URL: http://localhost:8080/api/sales\n")
}
