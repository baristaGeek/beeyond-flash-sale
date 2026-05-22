# Implementation Plan: Flash Sale Inventory Reservation System

**Branch**: `002-backend-setup` | **Date**: 2026-05-22 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-flash-sale-reservation/spec.md`

## Summary

A high-concurrency reservation service guaranteeing zero oversells under ≥100 simultaneous
requests. A Go backend serves a small REST API to a React/TypeScript dashboard; PostgreSQL
holds the authoritative state. Concurrent correctness is enforced by serializing every
reservation transaction on a pessimistically locked sale row (`SELECT ... FOR UPDATE`) and
deriving Available/Reserved counts from the normalized `reservations` table rather than from
a cached counter. A background TTL worker expires reservations within 1 second of their
60-second deadline. A Go-based load harness drives ≥100 parallel reservation requests against
a seeded sale and emits a pass/fail report on the no-oversell invariant.

## Technical Context

**Language/Version**: Go 1.22 (backend); TypeScript 5.4 (frontend)

**Primary Dependencies**:
- Backend: `github.com/go-chi/chi/v5` (router), `github.com/jackc/pgx/v5` (Postgres driver
  with `pgxpool` for connection pooling), `github.com/google/uuid` (IDs). No ORM.
- Frontend: React 18, Vite 5, TanStack Query (polling/caching). No UI component library
  beyond plain CSS modules.

**Storage**: PostgreSQL 16, run locally via the project's `docker-compose.yml`. Schema is
3NF and managed by versioned migrations under `backend/migrations/` (applied with the
`golang-migrate` CLI or an embedded equivalent).

**Testing**:
- Backend unit: `go test ./...` with the standard library.
- Backend integration: a separate test binary that spins up the same Docker Postgres,
  applies migrations, and runs concurrent goroutines against the real HTTP server.
- Load harness: `backend/loadtest/` — a small Go program that fires N goroutines at the
  reservation endpoint and prints a structured report (also exits non-zero on invariant
  violation, so CI can gate on it).
- Frontend: `vitest` + `@testing-library/react` for component-level; Playwright optional.

**Target Platform**: Linux server (production-shape); macOS/Linux developer machine running
Docker Desktop locally. Frontend targets modern evergreen browsers (Chromium, Firefox,
Safari).

**Project Type**: Web application (separate `backend/` and `frontend/` trees).

**Performance Goals**:
- Sustain ≥100 concurrent reservation requests against a single sale with zero oversells
  (SC-001, SC-007).
- TTL expiry within 60.0s + ≤1s (SC-002).
- Dashboard freshness ≤1s after underlying state change (SC-003).
- Bounded per-request latency: every reservation resolves to success or a typed conflict
  within a server-side lock-timeout budget (target 2s; reject with `LOCK_TIMEOUT` beyond
  that).

**Constraints**:
- Consistency over availability: under datastore unavailability or lock timeout, reject
  with a typed error; never serve stale or guessed inventory.
- No optimistic concurrency control (no version columns, no CAS loops) on inventory writes.
- No denormalized counters; Available = TotalCapacity − SUM(active reservation quantity).
- No ORM; SQL is hand-written and reviewed.
- No client-side authoritative state; the React app polls the server and reflects what
  it returns.

**Scale/Scope**: Portfolio-grade demo — one or a handful of active sales, capacities in
the 10–10,000-unit range, hundreds of concurrent users during load tests. Not designed
for global multi-region.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Plan compliance | Notes |
|---|-----------|-----------------|-------|
| I  | Consistency Over Availability (NON-NEGOTIABLE) | PASS | Lock-timeout and DB-unreachable paths return typed errors (`LOCK_TIMEOUT`, `INTERNAL`); no fallback to estimated stock. |
| II | Pessimistic Locking for Inventory Mutations (NON-NEGOTIABLE) | PASS | Reservation transaction does `SELECT id FROM sales WHERE id=$1 FOR UPDATE` before counting active reservations and inserting. No optimistic CAS anywhere. See `data-model.md`. |
| III | Normalized Relational Schema | PASS | Two tables (`sales`, `reservations`) in 3NF. Available count derived, not stored. FKs + CHECKs declared at the DB. |
| IV | Strict Stack Discipline | PASS | Backend: Go + Chi + pgx. DB: PostgreSQL via Docker Compose. Frontend: React + TypeScript + Vite. No other languages/frameworks. |
| V | Concurrency Correctness Is Tested, Not Assumed (NON-NEGOTIABLE) | PASS | `backend/loadtest/` and `backend/tests/integration/` exercise the real HTTP+Postgres stack with concurrent clients. CI runs both. |

**Initial gate**: PASS — no violations, no entries in Complexity Tracking.

*(Re-check after Phase 1 below.)*

## Project Structure

### Documentation (this feature)

```text
specs/001-flash-sale-reservation/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── api.md           # REST API contract
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
backend/
├── cmd/
│   └── server/
│       └── main.go               # HTTP server entry point
├── internal/
│   ├── config/                   # env/config loading
│   ├── db/                       # pgxpool wiring, transaction helpers
│   ├── inventory/                # inventory view query (Total/Reserved/Available)
│   ├── reservation/              # reservation create/release/expire core logic
│   ├── expirer/                  # background TTL worker
│   └── httpapi/                  # chi router, handlers, error mapping
├── migrations/                   # versioned SQL migrations (golang-migrate format)
├── loadtest/
│   └── main.go                   # concurrency proof harness; CLI flags for N, capacity
└── tests/
    ├── integration/              # real Postgres, real HTTP
    └── unit/                     # pure-Go logic tests

frontend/
├── src/
│   ├── api/                      # typed fetch wrapper + error-code mapping
│   ├── components/               # Dashboard, ReserveForm, ConflictState, etc.
│   ├── pages/                    # SalePage
│   ├── hooks/                    # useInventoryPoll, useReservationTimer
│   └── types/                    # shared TS types (matches backend response shapes)
├── public/
├── index.html
├── package.json
├── tsconfig.json
└── vite.config.ts

docker-compose.yml                # postgres service + (optionally) backend service
Makefile                          # convenience targets: up, migrate, test, loadtest
```

**Structure Decision**: Web application — `backend/` and `frontend/` trees at the repo
root, with `docker-compose.yml` coordinating local infrastructure. This matches the spec's
split between a Go service that owns the authoritative state and a React/TypeScript client
that renders it.

## Complexity Tracking

> No constitutional violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| _(none)_  | _(none)_   | _(none)_                              |

## Post-Design Constitution Re-check

After Phase 1 design (data-model.md, contracts/api.md, quickstart.md):

- **I. Consistency**: Confirmed. Every mutating endpoint opens a transaction at the handler
  boundary; lock-timeout and DB-down conditions surface as typed errors (`LOCK_TIMEOUT`,
  `INTERNAL`) rather than silent retries. See `contracts/api.md` error table.
- **II. Pessimistic locking**: Confirmed. The reservation-create flow's SQL is documented
  in `data-model.md` and uses `SELECT ... FOR UPDATE` on the `sales` row as the
  serialization anchor before reading active-reservation quantity and inserting. Release
  and TTL paths lock the reservation row.
- **III. Normalized schema**: Confirmed. `data-model.md` declares two 3NF tables with
  FKs, CHECKs, and indexes; Available is computed by query (`inventory view` block),
  never stored.
- **IV. Strict stack**: Confirmed. `contracts/api.md` and `quickstart.md` reference only
  Go/Chi/pgx, Postgres, and React/TS/Vite.
- **V. Concurrency tests**: Confirmed. `quickstart.md` documents both the integration test
  command and the load harness command; both run against the real Docker Postgres.

**Post-design gate**: PASS — design holds the constitutional line. Proceed to `/speckit-tasks`.
