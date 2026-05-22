# Quickstart: Flash Sale Inventory Reservation System

**Date**: 2026-05-22
**Plan**: [plan.md](./plan.md)

End-to-end runbook for spinning the system up locally, verifying it behaves correctly under
concurrency, and reproducing the load-test invariant report.

---

## Prerequisites

- Go 1.22+
- Node.js 20+ and npm (or pnpm)
- Docker Desktop (or any local Docker daemon supporting Compose v2)
- `make` (optional but assumed by the `make <target>` shortcuts below)
- `golang-migrate` CLI (`brew install golang-migrate` or equivalent)

No cloud accounts, no external services. Everything runs on `localhost`.

---

## 1. Bring up infrastructure

```bash
docker compose up -d postgres
```

This starts a single Postgres 16 container on port `5432` with the dev credentials in
`docker-compose.yml`. Wait a few seconds for the healthcheck to flip to ready, or:

```bash
docker compose ps
```

---

## 2. Apply migrations

```bash
make migrate-up
# or, directly:
migrate -path backend/migrations -database "$DATABASE_URL" up
```

`DATABASE_URL` defaults to `postgres://flashsale:flashsale@localhost:5432/flashsale?sslmode=disable`.

To reset between experiments:

```bash
make migrate-reset   # drops and recreates the schema
```

---

## 3. Seed a demo sale (recommended for UI development)

```bash
make seed
```

Inserts a deterministic demo sale (`id=00000001-0000-4000-8000-000000000000`,
capacity 100) so the frontend always has something to render. Idempotent: re-running
is a no-op. The frontend's default landing page points at this sale id when no
`?sale=<uuid>` is present in the URL.

## 4. Run the backend

```bash
cd backend
go run ./cmd/server
```

The server listens on `http://localhost:8080` by default. A background TTL worker starts
inside the same process and polls every 500ms.

---

## 5. Run the frontend

In a second terminal:

```bash
cd frontend
npm install
npm run dev
```

Vite serves the dashboard at `http://localhost:5173`, with API requests proxied to
`http://localhost:8080`.

---

## 6. Smoke test the API by hand

```bash
# Seed a sale with 10 units.
SALE_ID=$(curl -s -X POST http://localhost:8080/api/sales \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Sale","total_capacity":10}' | jq -r .id)

# Read inventory.
curl -s "http://localhost:8080/api/sales/$SALE_ID/inventory" | jq

# Reserve 3 units. Idempotency-Key is mandatory.
RES_ID=$(curl -s -X POST "http://localhost:8080/api/sales/$SALE_ID/reservations" \
  -H 'Content-Type: application/json' \
  -H 'X-Session-Id: 11111111-1111-1111-1111-111111111111' \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"quantity":3}' | jq -r .id)

# Inventory after reservation.
curl -s "http://localhost:8080/api/sales/$SALE_ID/inventory" | jq
# Expect: total_capacity=10, currently_reserved=3, available_to_reserve=7

# Release manually.
curl -s -X DELETE "http://localhost:8080/api/reservations/$RES_ID" \
  -H 'X-Session-Id: 11111111-1111-1111-1111-111111111111' | jq

# Inventory after release.
curl -s "http://localhost:8080/api/sales/$SALE_ID/inventory" | jq
# Expect: currently_reserved=0, available_to_reserve=10
```

---

## 7. Verify the headline guarantee: no oversells under concurrency

Run the load harness. This is the test that turns the no-oversell claim from
"we believe it works" into "we can prove it works."

```bash
cd backend
go run ./loadtest \
  --target http://localhost:8080 \
  --capacity 10 \
  --concurrency 100 \
  --per-request-quantity 1
```

**Expected output (abbreviated)**:

```
=== Flash Sale Load Test ===
Seeded sale: 7c2f... (capacity=10)
Fired:       100 reservation requests across 100 goroutines
Succeeded:   10
Conflicts:
  INSUFFICIENT_STOCK: 90
Final inventory:
  total_capacity:        10
  currently_reserved:    10
  available_to_reserve:  0
Invariant check:
  total = reserved + available .... HOLDS
  granted <= capacity ............ HOLDS
  no-oversell invariant .......... HOLDS
Exit: 0
```

If the no-oversell invariant ever prints `VIOLATED`, the harness exits non-zero, which is
the CI gate. This satisfies SC-001, SC-005, and SC-007.

Re-run with the same args; the resulting report should match line-for-line modulo timing
(SC-005, deterministic reports).

---

## 8. Verify TTL expiry

```bash
# Reserve, then wait without releasing.
curl -s -X POST "http://localhost:8080/api/sales/$SALE_ID/reservations" \
  -H 'Content-Type: application/json' \
  -H 'X-Session-Id: aaaa...' \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"quantity":5}'

# Watch inventory; ~60 seconds later, currently_reserved should drop back to 0.
watch -n 1 "curl -s http://localhost:8080/api/sales/$SALE_ID/inventory | jq"
```

SC-002: expiry within 60s + 1s tolerance.

---

## 9. Verify desync release (the edge case)

```bash
# Reserve, wait > 60s for auto-expiry, then attempt release.
RES_ID=$(curl -s -X POST "http://localhost:8080/api/sales/$SALE_ID/reservations" \
  -H 'X-Session-Id: bbbb...' -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"quantity":2}' | jq -r .id)

sleep 65

curl -s -X DELETE "http://localhost:8080/api/reservations/$RES_ID" \
  -H 'X-Session-Id: bbbb...' | jq
```

Expected: HTTP 200 with `status: "expired"` and `code: "RESERVATION_TERMINAL"`. No stack
trace, no double-return of stock. Verifies SC-006 and FR-014.

---

## 10. Run the test suites

```bash
# Pure unit tests (fast).
cd backend && go test ./internal/...

# Integration tests (real Postgres, real HTTP — must be brought up via docker compose).
go test ./tests/integration/...

# Race detector across all backend tests.
go test ./... -race

# Frontend type-check.
cd ../frontend && npm run typecheck

# Frontend tests.
npm test
```

CI runs all of the above plus the load harness (step 7) as a final gate.

---

## 11. Tear down

```bash
docker compose down -v   # -v wipes the Postgres volume; safe for dev.
```

---

## Troubleshooting

- **`pq: relation "reservations" does not exist`**: Run step 2 (`make migrate-up`).
- **`LOCK_TIMEOUT` storms during the load test**: Expected at very high concurrency on
  small capacities — that's the system fail-closing per Principle I. Reduce
  `--concurrency` or raise the server-side `lock_timeout`. The no-oversell invariant
  must still hold.
- **Dashboard counts disagree by ~1 second**: That's the polling interval; SC-003 allows
  ≤1s.
- **TTL appears to expire late**: Confirm the worker process is running (check server
  logs for `expirer: tick`) and that the host clock has not drifted; server time is
  authoritative.
