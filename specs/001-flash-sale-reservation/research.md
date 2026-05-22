# Phase 0 Research: Flash Sale Inventory Reservation System

**Date**: 2026-05-22
**Feature**: [spec.md](./spec.md)
**Plan**: [plan.md](./plan.md)

The Technical Context in `plan.md` had no `NEEDS CLARIFICATION` markers because the
constitution fixes the language, database, and frontend stack. The remaining decisions
below resolve the design-level choices that the constitution leaves open: which HTTP
router to use, which Go Postgres driver, what locking strategy to use against the
normalized schema, and how to drive deterministic TTL expiry.

---

## R1 — HTTP router

**Decision**: `github.com/go-chi/chi/v5`.

**Rationale**:
- The constitution allows stdlib, Chi, or Gin. Chi gives us idiomatic `http.Handler`
  composition (every Chi handler is a plain `http.HandlerFunc`), trivial middleware, and
  URL param routing without dragging in a parallel reflection-driven framework.
- Tests can hit handlers directly via `httptest.NewRecorder` without any framework-specific
  test harness.
- Zero runtime dependencies beyond the standard library.

**Alternatives considered**:
- **`net/http` stdlib only**: Possible, but URL param parsing and middleware composition
  would need hand-rolled helpers; doesn't add value over Chi.
- **Gin**: Faster in microbenchmarks, but its custom `Context` complicates wrapping the
  handler in a SQL transaction at the boundary (a constitutional requirement) and adds
  more dependency surface than we need at this scale.

---

## R2 — PostgreSQL driver and transaction model

**Decision**: `github.com/jackc/pgx/v5` accessed via the `pgxpool` connection pool and
`pgx.Tx`. No ORM. Hand-written SQL in small repository-style helpers.

**Rationale**:
- pgx is the de facto modern driver: better PostgreSQL type support, native protocol,
  faster than `lib/pq`, and actively maintained.
- Using `pgxpool` directly (rather than `database/sql`) keeps transaction boundaries
  explicit: handlers acquire a connection, call `BeginTx`, pass the `pgx.Tx` into the
  reservation service, and commit/rollback before responding. This matches the
  constitutional rule "Transactions at the boundary."
- Hand-written SQL keeps the locking story auditable from the source alone — exactly what
  Principle II demands.

**Alternatives considered**:
- **`database/sql` + pgx stdlib adapter**: Slightly more portable but loses pgx-specific
  helpers and adds an indirection for no benefit in this single-driver project.
- **GORM / sqlc / ent**: ORMs hide locking semantics and tempt the codebase toward
  patterns (auto-retries, magic transactions) that conflict with Principle II.

---

## R3 — Concurrency strategy under a normalized schema

**Decision**: Pessimistic row lock on the `sales` row as the serialization anchor for every
reservation mutation. Available stock is derived inside the locked transaction.

**Transaction shape (reservation create)**:

```sql
BEGIN;
SET LOCAL lock_timeout = '2s';

-- 1. Serialize all reservation mutations against this sale.
SELECT id, total_capacity
  FROM sales
 WHERE id = $1
   FOR UPDATE;

-- 2. Compute current reserved quantity inside the lock.
SELECT COALESCE(SUM(quantity), 0) AS reserved
  FROM reservations
 WHERE sale_id = $1 AND status = 'active';

-- 3. If total_capacity - reserved >= requested, insert; else return INSUFFICIENT_STOCK.
INSERT INTO reservations (id, sale_id, session_id, quantity, created_at, expires_at, status)
VALUES ($2, $1, $3, $4, now(), now() + interval '60 seconds', 'active');

COMMIT;
```

**Rationale**:
- Without a denormalized stock counter (forbidden by the constitution), the natural
  serialization point is the parent `sales` row. `SELECT ... FOR UPDATE` on that row
  forces all concurrent reservation transactions to queue, eliminating the race window
  between "count active reservations" and "insert new reservation."
- `SET LOCAL lock_timeout` bounds wait time; on expiry, the transaction errors out and we
  surface `LOCK_TIMEOUT` per Principle I (fail closed, never guess).
- Release and TTL-expiry transactions also `SELECT ... FOR UPDATE` the relevant
  `reservations` row (not the sale row) so that release-vs-expire races serialize on the
  reservation itself; each path checks `status = 'active'` before mutating and exits as a
  no-op if it lost the race. The `sales` row is only locked when changes affect derived
  inventory (i.e., creating a new active reservation).

**Alternatives considered**:
- **Optimistic version column on `sales` with CAS retry**: Forbidden by Principle II.
- **`SERIALIZABLE` isolation everywhere**: Achieves the same correctness but degrades
  more sharply under contention (serialization failures cascade as retries, which is
  hostile to the "no implicit retry" stance of Principle I).
- **Advisory locks (`pg_advisory_xact_lock`)**: Works, but ties correctness to a key
  scheme outside the schema; row locks tie correctness to the row that semantically
  owns the invariant.

---

## R4 — TTL expiry mechanism

**Decision**: A background worker goroutine inside the backend process polls every 500ms
for `reservations WHERE status='active' AND expires_at <= now()`, and transitions each to
`status='expired'` inside its own short transaction (locking the reservation row).

**Rationale**:
- The spec requires expiry within 60.0s + 1s tolerance (SC-002). A 500ms poll interval
  gives a worst-case latency of ~500ms + transaction time, well inside the budget.
- Co-locating the worker with the API process avoids a separate scheduler, cron, or
  external job runner — fewer moving parts for a portfolio-scale deployment.
- The worker's transaction shape mirrors manual release (lock the reservation row,
  check `status='active'`, update to `expired`, set `expired_at`), so the release-vs-expiry
  race is serialized by the database, satisfying FR-015.

**Alternatives considered**:
- **Lazy expiry on read** (check `expires_at < now()` at every query and treat as expired):
  Would defer the Available-count return until the next read. SC-002 demands the
  Available pool actually rises within 1s of expiry, including when no client is reading;
  lazy expiry fails this for idle periods.
- **`pg_cron` / SQL-side scheduler**: Adds a Postgres extension dependency for one task.
- **`LISTEN/NOTIFY` driven**: Complex; doesn't help expiry, which is time-based, not
  event-based.

---

## R5 — Frontend data flow and freshness

**Decision**: TanStack Query polling at a 1-second interval for the inventory endpoint;
typed `fetch` wrapper that translates server error codes into a discriminated union for
the UI to render conflict states from.

**Rationale**:
- 1-second polling trivially meets SC-003 (dashboard freshness ≤1s) and avoids the
  operational complexity of SSE/WebSockets for a portfolio project.
- TanStack Query handles caching, stale-while-revalidate, and request deduplication out
  of the box — keeps component code thin.
- A discriminated union of error codes (`INSUFFICIENT_STOCK | INVALID_QUANTITY |
  RESERVATION_TERMINAL | RESERVATION_NOT_FOUND | LOCK_TIMEOUT | INTERNAL`) maps each
  backend code to a dedicated visual state, satisfying FR-016/FR-017.

**Alternatives considered**:
- **SSE / WebSocket push**: More "real-time," but adds connection-state complexity for
  marginal UX gain at this scale.
- **Manual `setInterval` + fetch**: Possible, but reinventing what TanStack Query already
  does well (and tested).

---

## R6 — Load test harness design

**Decision**: A standalone Go program at `backend/loadtest/main.go` that:
1. Takes CLI flags `--capacity`, `--concurrency`, `--per-request-quantity`, `--target`.
2. Calls a seed endpoint to create a fresh sale at the requested capacity.
3. Fires `--concurrency` goroutines, each issuing a single reservation request via
   `net/http`. Uses `sync.WaitGroup` to join, collects per-request outcomes.
4. After all goroutines complete, queries the inventory endpoint to read final Total,
   Reserved, Available.
5. Prints a structured report (JSON + human-readable) including: requests fired,
   successes, conflict counts by code, final inventory, and an explicit
   `no-oversell invariant: HOLDS|VIOLATED` line.
6. Exits non-zero on violation so CI can gate.

**Rationale**:
- A pure-Go harness is reproducible, fast to start, and runs in CI without external
  tools. Matches FR-020 through FR-022 and SC-005.
- Using real HTTP (not in-process handler calls) exercises the full network → router →
  transaction stack, which is what the constitution's Principle V demands.

**Alternatives considered**:
- **`k6` / `vegeta` / `wrk`**: Off-the-shelf load tools, but they don't natively assert
  on database invariants. We'd still need a Go program to compute pass/fail.
- **Go benchmarks (`go test -bench`)**: Awkward for concurrency-oriented invariant checks
  and doesn't produce a standalone report.

---

## R7 — Identifiers and timestamps

**Decision**: UUIDv4 for `sales.id` and `reservations.id`. All timestamps `timestamptz`,
generated server-side via `now()`.

**Rationale**:
- UUIDs avoid enumeration of reservation IDs and remove a coordination point (sequence
  generation) under concurrency.
- Server-side `now()` makes the database the authority for TTL math, consistent with
  the spec's "Server time is authoritative" assumption.

**Alternatives considered**:
- **BIGSERIAL IDs**: Smaller, ordered, but expose volume and require coordination on
  insert. Not worth the trade for a demo.

---

## Open questions for `/speckit-clarify` (optional)

None block planning. All Phase 0 unknowns are resolved. The remaining items that could
*optionally* be clarified live in the spec's Assumptions section — most notably the
"no checkout/confirm step in v1" decision — and can be revisited if the user wants
to expand scope before tasks are generated.
