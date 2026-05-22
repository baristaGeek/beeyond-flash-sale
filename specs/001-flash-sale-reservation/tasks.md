---
description: "Task list for the Flash Sale Inventory Reservation System feature"
---

# Tasks: Flash Sale Inventory Reservation System

**Input**: Design documents from `/specs/001-flash-sale-reservation/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/api.md`,
`quickstart.md`

**Tests**: Tests are INCLUDED in this task list because Constitution Principle V
("Concurrency Correctness Is Tested, Not Assumed") mandates concurrent integration tests
against a real PostgreSQL instance for every inventory-mutating feature. Frontend unit
tests are not generated; manual verification per `quickstart.md` is the v1 sign-off.

**Organization**: Tasks are grouped by user story so each story can be implemented and
verified as an independent increment.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label (US1…US6); omitted in Setup, Foundational, and Polish phases
- Every task includes the exact file path it touches

## Path Conventions

This is a **web application** project per `plan.md`:

- Backend: `backend/cmd/`, `backend/internal/`, `backend/migrations/`, `backend/loadtest/`,
  `backend/tests/`
- Frontend: `frontend/src/`
- Infrastructure: `docker-compose.yml`, `Makefile` at repo root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure. No business logic yet.

- [X] T001 Initialize the Go module at the repository root for the backend: `cd backend && go mod init github.com/baristaGeek/beeyond-flash-sale/backend`, producing `backend/go.mod` and `backend/go.sum`.
- [X] T002 [P] Scaffold the React + TypeScript + Vite frontend in `frontend/` via `npm create vite@latest frontend -- --template react-ts`; commit `frontend/package.json`, `frontend/tsconfig.json`, `frontend/vite.config.ts`, `frontend/index.html`, and the generated `frontend/src/main.tsx` / `frontend/src/App.tsx`.
- [X] T003 [P] Author `docker-compose.yml` at the repo root defining a single `postgres` service on port 5432 (image `postgres:16`, env `POSTGRES_USER=flashsale`, `POSTGRES_PASSWORD=flashsale`, `POSTGRES_DB=flashsale`) with a `pg_isready` healthcheck and a named volume.
- [X] T004 [P] Author `Makefile` at the repo root with targets `up`, `down`, `migrate-up`, `migrate-down`, `migrate-reset`, `run-backend`, `run-frontend`, `test`, `test-race`, `loadtest`, mirroring the commands in `specs/001-flash-sale-reservation/quickstart.md`.
- [X] T005 [P] Add `backend/.golangci.yml` enabling `errcheck`, `govet`, `staticcheck`, `gosimple`, `revive`, `gofmt`, and `goimports`; wire `make lint-backend` to run it.
- [X] T006 [P] Enable TypeScript strict mode in `frontend/tsconfig.json` (`"strict": true`, `"noUncheckedIndexedAccess": true`) and add `frontend/.eslintrc.cjs` + `frontend/.prettierrc` for consistent style; wire `make lint-frontend`.
- [X] T007 [P] Add `frontend/vite.config.ts` proxy config so `/api/*` is forwarded to `http://localhost:8080` during `npm run dev`.

**Checkpoint**: Repo has Go module, frontend scaffold, Postgres compose, Makefile, and linters. Nothing runnable end-to-end yet.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database schema, connection pool, HTTP routing skeleton, error envelope, and shared frontend plumbing. Must be complete before any user story can be implemented.

**CRITICAL**: No user story work can begin until this phase is complete.

### Database & migrations

- [X] T008 Write migration `backend/migrations/0001_init.up.sql` creating the `reservation_status` enum, the `sales` table, and the `reservations` table with all CHECK constraints, the partial indexes (`reservations_sale_active_idx`, `reservations_active_expiring_idx`, `reservations_session_idx`), and the cross-column status/timestamp CHECK exactly as specified in `data-model.md`.
- [X] T009 Write the inverse migration `backend/migrations/0001_init.down.sql` that drops the two tables and the enum (for local resets).
- [X] T010 Write migration `backend/migrations/0002_idempotency.up.sql` creating the `idempotency_records` table with PK `(idempotency_key, session_id)`, FKs to `sales` (`ON DELETE CASCADE`) and `reservations` (`ON DELETE SET NULL`), CHECK on key length, and the `idempotency_records_gc_idx` on `created_at`, per `data-model.md`.
- [X] T011 Write the inverse migration `backend/migrations/0002_idempotency.down.sql` that drops `idempotency_records`.

### Backend foundation

- [X] T012 [P] Implement `backend/internal/config/config.go` loading `DATABASE_URL`, `HTTP_ADDR` (default `:8080`), `LOCK_TIMEOUT` (default `2s`), `EXPIRER_POLL_INTERVAL` (default `500ms`), and `IDEMPOTENCY_RETENTION` (default `24h`) from environment variables with sane defaults and a `MustLoad()` entry point.
- [X] T013 [P] Implement `backend/internal/db/pool.go` exposing `New(ctx, cfg) (*pgxpool.Pool, error)` and a `WithTx(ctx, pool, opts, fn)` helper that opens a transaction, runs `SET LOCAL lock_timeout = ...`, calls `fn(tx)`, and commits or rolls back — this is the boundary-level transaction helper that Constitution Principle I requires every mutating handler to use.
- [X] T014 [P] Implement `backend/internal/db/migrate.go` wrapping `golang-migrate` (or `embed`+`pressly/goose`) so `make migrate-up` and `make migrate-down` work against `DATABASE_URL` and load files from `backend/migrations/`.
- [X] T015 Implement `backend/internal/httpapi/errors.go` defining the `ErrorCode` string type with the constants `INVALID_QUANTITY`, `SALE_NOT_FOUND`, `RESERVATION_NOT_FOUND`, `RESERVATION_TERMINAL`, `INSUFFICIENT_STOCK`, `IDEMPOTENCY_KEY_MISMATCH`, `LOCK_TIMEOUT`, `VALIDATION`, `INTERNAL`; an `APIError` struct with `Code`, `Message`, `Details`; and a `WriteError(w, status, err)` helper that emits the JSON envelope defined in `contracts/api.md`.
- [X] T016 Implement `backend/internal/httpapi/middleware.go` with: `RequireSession` (validates and injects `X-Session-Id`), `Recover` (panic-to-INTERNAL), `RequestLogger` (structured slog), and `JSONContentType`.
- [X] T017 Implement `backend/internal/httpapi/router.go` constructing the `chi.Router` with the middleware chain from T016 and a `New(pool, cfg) http.Handler` factory; route registrations are stubbed and will be filled by story phases.
- [X] T018 Implement `backend/cmd/server/main.go` wiring config → pool → migrate-on-boot (optional flag) → router → `http.Server` with graceful shutdown on SIGINT/SIGTERM; background workers will hook in here in later phases.

### Frontend foundation

- [X] T019 [P] Implement `frontend/src/types/api.ts` mirroring the TypeScript types in `contracts/api.md` (`Inventory`, `Reservation`, `ReservationStatus`, `ApiErrorCode`, `ApiError`) — the `ApiErrorCode` union MUST include `IDEMPOTENCY_KEY_MISMATCH`.
- [X] T020 [P] Implement `frontend/src/api/client.ts` exposing a typed `fetch` wrapper (`apiGet`, `apiPost`, `apiDelete`) that attaches the `X-Session-Id` header from `localStorage` (generated on first visit via `crypto.randomUUID()`), accepts an optional `idempotencyKey` for `apiPost`, and parses JSON error envelopes into a typed `ApiErrorEnvelope` discriminated union.
- [X] T021 [P] Implement `frontend/src/api/errors.ts` exposing a `isApiError(e): e is ApiError` type guard and a default human-readable label table per `ApiErrorCode`, leaving room for component-level overrides.
- [X] T022 Mount the TanStack Query `QueryClientProvider` in `frontend/src/main.tsx` and add a basic `App.tsx` layout shell that will host the SalePage once US2 lands.

**Checkpoint**: Backend builds, Postgres + migrations run cleanly, HTTP server boots and serves `/healthz`. Frontend builds, types are aligned with the contract. User story work can now begin in parallel.

---

## Phase 3: User Story 1 - Atomic Reservation Under Contention (Priority: P1) 🎯 MVP

**Goal**: A user can request to reserve N units against a sale and the system either grants exactly N units atomically or rejects the request with a typed conflict. Under 100 concurrent requests, the system never grants more units than the sale's Total Capacity. Repeat requests carrying the same `Idempotency-Key` return the same outcome and never decrement stock twice.

**Independent Test**: Seed a sale with capacity 10. Fire 100 concurrent reservation requests for 1 unit each. Verify exactly 10 succeed and 90 receive `INSUFFICIENT_STOCK`. Submit the *same* successful request twice with the same `Idempotency-Key` and verify the second call returns the original reservation ID without a second decrement. Submit two requests with the same key but different quantities and verify the second is rejected with `IDEMPOTENCY_KEY_MISMATCH`.

### Tests for User Story 1 (per Constitution Principle V)

> Write these tests FIRST. They MUST fail before implementation begins. They MUST run against the real Docker Postgres, not a mock.

- [X] T023 [P] [US1] Concurrent reservation integration test in `backend/tests/integration/reservation_concurrency_test.go`: spins up an HTTP server bound to a test pool, seeds a sale at capacity 10, fires 100 goroutines (each posting to `POST /api/sales/{id}/reservations` with quantity 1), asserts exactly 10 successes and 90 `INSUFFICIENT_STOCK`, and asserts the final `currently_reserved` is exactly 10 (no oversells, no undersells).
- [X] T024 [P] [US1] Idempotency replay integration test in `backend/tests/integration/idempotency_replay_test.go`: covers three cases — (a) same key + same payload twice → same reservation ID, stock decremented once; (b) same key + different quantity → second call returns `IDEMPOTENCY_KEY_MISMATCH` and stock unchanged; (c) idempotent replay of a rejected request (`INSUFFICIENT_STOCK`) → second call returns the same rejection, stock unchanged.
- [X] T025 [P] [US1] Idempotency concurrency test in `backend/tests/integration/idempotency_concurrency_test.go`: fires 10 goroutines posting the *same* payload with the *same* `Idempotency-Key` and asserts exactly one reservation row exists, exactly one stock decrement occurred, and all 10 responses carry the same reservation ID.

### Implementation for User Story 1

- [X] T026 [P] [US1] Implement `backend/internal/reservation/hash.go` exposing `CanonicalRequestHash(saleID uuid.UUID, body []byte) []byte` returning `SHA-256(sale_id_bytes || canonical_json_of_body)`. Canonical JSON: keys sorted lexicographically, no whitespace.
- [X] T027 [P] [US1] Implement `backend/internal/reservation/types.go` with the `Reservation` struct mirroring the schema, the `Status` typed string, and the `CreateRequest` / `CreateResponse` DTOs.
- [X] T028 [US1] Implement the consistency-critical core in `backend/internal/reservation/create.go`: `Create(ctx, tx, params) (*Reservation, error)` runs `SELECT id, total_capacity FROM sales WHERE id=$1 FOR UPDATE`, computes `SUM(quantity)` of active reservations, and either inserts the new reservation row (status `active`, `expires_at = now() + 60s`) or returns a typed `ErrInsufficientStock{Available: int}` — exactly the SQL in `data-model.md § reservations.create`.
- [X] T029 [US1] Implement `backend/internal/reservation/idempotency.go` exposing `WithIdempotency(ctx, tx, key, sessionID, saleID, requestHash, run func() (status int, body []byte, reservationID *uuid.UUID, err error)) (cached bool, status int, body []byte, err error)` implementing the `INSERT ... ON CONFLICT DO NOTHING RETURNING` claim-or-observe pattern, branch A (winner: runs `run`, persists the response), branch B (loser: `SELECT ... FOR UPDATE` on the existing row, compares hashes, replays cached or returns `ErrIdempotencyKeyMismatch{OriginalHash, SubmittedHash}`).
- [X] T030 [US1] Implement `backend/internal/httpapi/sales.go` registering `POST /api/sales` (admin/seed): validates body, opens a transaction, inserts a `sales` row with a new UUID, returns the 201 response shape from `contracts/api.md § Endpoint 1`.
- [X] T031 [US1] Implement `backend/internal/httpapi/reservations.go::HandleCreate` registering `POST /api/sales/{sale_id}/reservations`: parses `X-Session-Id` (required), parses optional `Idempotency-Key` header, parses body, opens a transaction with `lock_timeout = 2s`, dispatches via the idempotency helper from T029 wrapping `reservation.Create` from T028, and maps each typed error to the HTTP status + error code table in `contracts/api.md`. Wire the route in `backend/internal/httpapi/router.go` (T017).
- [X] T032 [US1] Add per-error mapping for `LOCK_TIMEOUT` (detect `pgconn.PgError` code `55P03`) in `backend/internal/httpapi/errors.go` so timeouts surface as HTTP 503 with code `LOCK_TIMEOUT` (Constitution Principle I — fail closed).

### Frontend for User Story 1

- [X] T033 [P] [US1] Implement `frontend/src/hooks/useReservation.ts`: a TanStack Query mutation hook that calls `apiPost("/api/sales/{id}/reservations", body, { idempotencyKey })`, generates the key with `crypto.randomUUID()` on the first attempt and *reuses* it on retries (so network-retry replays hit the same cached response), and returns a typed `{ data, error, isPending }`.
- [X] T034 [P] [US1] Implement `frontend/src/components/ReserveForm.tsx`: a controlled quantity input with submit; on success shows the reservation ID and expiry; on `INSUFFICIENT_STOCK` shows `details.available_to_reserve`; on `IDEMPOTENCY_KEY_MISMATCH` shows a calm "start a new attempt" message and resets the local key.
- [X] T035 [US1] Implement `frontend/src/pages/SalePage.tsx` skeleton: accepts a `sale_id` (URL param or constant for the v1 demo), renders `<ReserveForm/>`; later phases add the Dashboard and Timer to this page.

**Checkpoint (MVP)**: At this point the headline guarantee is provable. T023 passes (no oversells under 100 concurrent), T024+T025 pass (idempotent replays). The product can be demoed: a user reserves units, the database holds the invariant, and the load test (US6) will later certify this in CI.

---

## Phase 4: User Story 2 - Live Inventory Dashboard (Priority: P1)

**Goal**: Anyone viewing the page sees the current `Total`, `Currently Reserved`, and `Available to Reserve` counts, updated within 1 second of any underlying state change.

**Independent Test**: Open the dashboard. Seed a reservation directly via the API in a second terminal. Verify the dashboard reflects the new Reserved/Available counts within 1 second.

### Implementation for User Story 2

- [ ] T036 [US2] Implement `backend/internal/inventory/inventory.go::GetBySaleID(ctx, pool, saleID) (*View, error)` running the derived-view query in `data-model.md § Derived view: inventory state`, returning `{SaleID, TotalCapacity, CurrentlyReserved, AvailableToReserve}`. No transactional lock — best-effort read.
- [ ] T037 [US2] Implement `backend/internal/httpapi/inventory.go::HandleGet` registering `GET /api/sales/{sale_id}/inventory` returning the response shape in `contracts/api.md § Endpoint 2`; map `SaleNotFound` to HTTP 404 / `SALE_NOT_FOUND`. Wire the route in `backend/internal/httpapi/router.go`.

### Frontend for User Story 2

- [ ] T038 [P] [US2] Implement `frontend/src/hooks/useInventoryPoll.ts` as a TanStack Query `useQuery` with `refetchInterval: 1000`, `staleTime: 0`, querying `GET /api/sales/{id}/inventory`.
- [ ] T039 [P] [US2] Implement `frontend/src/components/Dashboard.tsx` rendering Total / Currently Reserved / Available as three labeled tiles, with an "updating…" indicator during refetch, and a subtle "as of <timestamp>" line.
- [ ] T040 [US2] Wire `<Dashboard/>` into `frontend/src/pages/SalePage.tsx` above the `<ReserveForm/>`.

**Checkpoint**: Customers see live stock and can reserve. The two P1 customer-facing capabilities (reserve + see-state) are both functional.

---

## Phase 5: User Story 3 - Deterministic 60-Second TTL Expiry (Priority: P1)

**Goal**: Every reservation auto-expires within 60s + 1s of its creation; its units return to the Available pool exactly once; the reservation row is retained as a tombstone with `status='expired'`.

**Independent Test**: Create a reservation, do nothing, and verify within 61 seconds that the reservation's status is `expired`, `expired_at` is non-null, and the parent sale's `available_to_reserve` count has returned to its pre-reservation value.

### Tests for User Story 3

- [ ] T041 [P] [US3] TTL expiry integration test in `backend/tests/integration/expiry_test.go`: creates a reservation with a configurable shortened TTL (override via test-only config or by directly setting `expires_at` to `now() + 1s`), waits 2 seconds, asserts the reservation row has `status='expired'` and that the inventory view's `available_to_reserve` has rebounded.

### Implementation for User Story 3

- [ ] T042 [US3] Implement `backend/internal/reservation/expire.go::ExpireOne(ctx, tx, reservationID)` running `SELECT id, status FROM reservations WHERE id=$1 FOR UPDATE` then `UPDATE ... SET status='expired', expired_at=now() WHERE id=$1 AND status='active'` — a no-op if release won the race.
- [ ] T043 [US3] Implement `backend/internal/expirer/expirer.go::Worker` that ticks every `cfg.ExpirerPollInterval` (default 500ms), selects up to 100 candidates with `SELECT id FROM reservations WHERE status='active' AND expires_at <= now() ORDER BY expires_at LIMIT 100`, and processes each in its own transaction via `expire.ExpireOne`. Provides `Start(ctx)` and `Stop()` for lifecycle, logs each tick at debug level.
- [ ] T044 [US3] Wire the expirer worker into `backend/cmd/server/main.go`: start it after the pool is ready, stop it on shutdown, before closing the pool.

### Frontend for User Story 3

- [ ] T045 [P] [US3] Implement `frontend/src/components/ReservationTimer.tsx`: takes the reservation's `expires_at` ISO timestamp, renders a countdown updated every 250ms purely from `Date.now()`, and when the timer hits zero it does NOT auto-mutate state — it triggers a refetch of the reservation via `GET /api/reservations/{id}` (per `contracts/api.md § Endpoint 5`) so the server remains authoritative.
- [ ] T046 [US3] After a successful reservation in `ReserveForm.tsx`, render `<ReservationTimer/>` next to the reservation summary (US1 follow-up).

**Checkpoint**: The flash sale dynamic is live — held inventory flows back into the pool on a deterministic schedule.

---

## Phase 6: User Story 4 - Manual Release With Desync Handling (Priority: P2)

**Goal**: Users can release an active reservation early; releases against already-terminal reservations are idempotent and graceful; the release-vs-TTL-expiry race is correctly serialized.

**Independent Test**: (a) Create a reservation, release it before T+60s, verify stock returns exactly once. (b) Create a reservation, sleep past T+60s, then submit release — verify response 200 with `code: RESERVATION_TERMINAL`, no stack trace, stock unchanged from its post-expiry value.

### Tests for User Story 4

- [ ] T047 [P] [US4] Manual release integration test in `backend/tests/integration/release_test.go`: covers happy-path release of an active reservation (stock returns once), idempotent release-after-release (200 with `RESERVATION_TERMINAL`), and release-of-unknown-id (404 `RESERVATION_NOT_FOUND`).
- [ ] T048 [P] [US4] Release-after-expiry (desync) integration test in `backend/tests/integration/release_desync_test.go`: creates a reservation with `expires_at = now() + 1s`, waits for the expirer to tick past it, then submits a release. Asserts 200, code `RESERVATION_TERMINAL`, status `expired`, and no double-return of stock.
- [ ] T049 [P] [US4] Release-vs-expire race integration test in `backend/tests/integration/release_expire_race_test.go`: creates 50 reservations all with `expires_at = now() + 200ms`, fires a manual `DELETE` against each *exactly* when the expirer is about to fire; asserts that for each reservation, exactly one of (`released`, `expired`) wins, the units are returned exactly once, and the losing path is a clean no-op.

### Implementation for User Story 4

- [ ] T050 [US4] Implement `backend/internal/reservation/release.go::Release(ctx, tx, reservationID, sessionID) (*Reservation, error)` running `SELECT ... FOR UPDATE`, checks `session_id == caller`, checks `status == 'active'`. Returns `ErrReservationNotFound` for missing/foreign-session, `ErrReservationTerminal` for already-terminal (with the current row so the caller can echo `status` back). On the happy path: `UPDATE ... SET status='released', released_at=now()`.
- [ ] T051 [US4] Implement `backend/internal/httpapi/reservations.go::HandleDelete` registering `DELETE /api/reservations/{reservation_id}` per `contracts/api.md § Endpoint 4`. Maps `ErrReservationTerminal` to HTTP 200 with `code: RESERVATION_TERMINAL` (idempotent path, not an error status); maps `ErrReservationNotFound` to HTTP 404. Wire the route.
- [ ] T052 [US4] Implement `backend/internal/httpapi/reservations.go::HandleGet` registering `GET /api/reservations/{reservation_id}` per `contracts/api.md § Endpoint 5`, returning the full reservation row for timer resync. Wire the route.

### Frontend for User Story 4

- [ ] T053 [P] [US4] Implement `frontend/src/hooks/useReleaseReservation.ts`: TanStack Query mutation hook calling `apiDelete("/api/reservations/{id}")`; on success or on `RESERVATION_TERMINAL` it invalidates the inventory query so the dashboard refreshes.
- [ ] T054 [P] [US4] Implement `frontend/src/components/ReleaseButton.tsx`: button that submits the release; on `RESERVATION_TERMINAL` response, shows a calm "this reservation was already released or expired" message instead of an error.

**Checkpoint**: Users have full control of their reservations and the system gracefully handles the timer-desync edge case that previously would have produced either a double-return bug or a scary error.

---

## Phase 7: User Story 5 - Graceful Conflict Mapping (Priority: P2)

**Goal**: Every backend rejection surfaces as a distinct, friendly UI state. No raw HTTP status codes, JSON bodies, or stack traces ever appear in user-visible surfaces.

**Independent Test**: Force each of the 9 error codes from the backend (validate, sale-not-found, reservation-not-found, reservation-terminal, insufficient-stock, idempotency-key-mismatch, lock-timeout, internal) and verify the UI renders a distinct visual state for each, never a raw code.

- [ ] T055 [P] [US5] Audit and harden `backend/internal/httpapi/*.go` so that every code path that can fail emits a typed `ErrorCode` via `WriteError`. Add a fall-through `Recover` middleware in `backend/internal/httpapi/middleware.go` that converts any unhandled panic or untyped error into `INTERNAL` (HTTP 500) without leaking stack traces.
- [ ] T056 [P] [US5] Implement `frontend/src/components/ConflictState.tsx`: a single component accepting an `ApiErrorCode` discriminant and rendering a per-code variant (icon + headline + body + suggested next action). One variant per code in the union from `frontend/src/types/api.ts`.
- [ ] T057 [US5] Wire `<ConflictState code={...} details={...}/>` into `ReserveForm.tsx` and `ReleaseButton.tsx` in place of any generic error rendering. Ensure no `JSON.stringify(error)` or raw `e.message` strings reach the DOM.

**Checkpoint**: The UI feels like a product, not a debugger. Every conflict reads as part of the normal flash-sale flow.

---

## Phase 8: User Story 6 - Provable Concurrency Correctness via Load Testing (Priority: P3)

**Goal**: A single command demonstrates ≥100 concurrent reservation requests producing zero oversells, and emits a deterministic, machine-readable pass/fail report.

**Independent Test**: Run `make loadtest` after `make up && make migrate-up`. Verify the printed report shows exactly `capacity` successes, `concurrency - capacity` conflicts, and `no-oversell invariant: HOLDS`. The process exits 0. Re-run on the same seed and verify the report is identical modulo timestamps.

- [ ] T058 [US6] Implement `backend/loadtest/main.go` with CLI flags `--target`, `--capacity`, `--concurrency`, `--per-request-quantity` (defaults match `quickstart.md § Step 6`). On start: POSTs to `/api/sales` to seed a fresh sale; spawns `--concurrency` goroutines each issuing one reservation request via `net/http`; joins via `sync.WaitGroup`; queries `/api/sales/{id}/inventory` for final state.
- [ ] T059 [US6] Implement the report struct + emitter in `backend/loadtest/report.go`: prints a human-readable section and a final JSON line; computes the invariant `granted_units ≤ capacity` and `total = reserved + available`; prints `no-oversell invariant: HOLDS|VIOLATED`; exits with code 1 on `VIOLATED` so CI can gate.
- [ ] T060 [P] [US6] Add a `loadtest` Makefile target chaining `make up`, `make migrate-up`, build, and run with default flags (capacity 10, concurrency 100).
- [ ] T061 [P] [US6] Append a "Load test" section to `README.md` (creating the file if needed) referencing `quickstart.md § Step 6` and showing the expected output.

**Checkpoint**: The headline claim of the system — "no oversells under concurrency" — is now a reproducible, CI-gatable fact, not a belief.

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Cleanup and operational concerns that span all stories. None block the MVP.

- [ ] T062 Implement `backend/internal/expirer/idempotency_gc.go::SweepIdempotencyRecords(ctx, pool, retention time.Duration)` running `DELETE FROM idempotency_records WHERE created_at < now() - $1` on a longer interval (default every 60s); wire it into `backend/cmd/server/main.go` alongside the TTL expirer. Honors FR-026's 24h retention floor.
- [ ] T063 [P] Add `go test ./... -race` and `go test ./tests/integration/...` to a CI config file (`.github/workflows/ci.yml` if GitHub; otherwise document the command set in `README.md`).
- [ ] T064 [P] Add `make loadtest` as a CI gate step so the no-oversell invariant is asserted on every push (using a 10-capacity / 50-concurrency profile to keep CI fast).
- [ ] T065 [P] Add structured request logging (request ID, route, status, duration, error code) to `backend/internal/httpapi/middleware.go::RequestLogger` so post-incident traceability (Constitution governance "Traceability") is non-trivial without running the database.
- [ ] T066 Run the full `specs/001-flash-sale-reservation/quickstart.md` end-to-end on a clean machine and resolve any documentation gaps; record the run as the v1 acceptance.
- [ ] T067 [P] Update `README.md` with a one-page project overview linking to `.specify/memory/constitution.md`, `specs/001-flash-sale-reservation/spec.md`, and `specs/001-flash-sale-reservation/plan.md`.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: No dependencies. Start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1. BLOCKS all user stories.
- **US1 (Phase 3, P1)**: Depends on Phase 2. Independent of other stories. **MVP.**
- **US2 (Phase 4, P1)**: Depends on Phase 2. Independent of US1 functionally but the dashboard only has interesting numbers to display once US1 lands; recommended to start US2 in parallel with US1 once Phase 2 ships.
- **US3 (Phase 5, P1)**: Depends on Phase 2. The release-vs-expire race tests in US4 depend on US3 being merged (the expirer must exist), so US3 should precede or run in parallel with US4.
- **US4 (Phase 6, P2)**: Depends on Phase 2; the desync and race tests additionally depend on US3.
- **US5 (Phase 7, P2)**: Depends on US1, US2, and US4 because it audits the error paths each of those introduces.
- **US6 (Phase 8, P3)**: Depends on US1 (it drives the reservation endpoint) and US2 (it reads the inventory endpoint for final-state verification).
- **Polish (Phase N)**: Depends on all desired user stories being merged.

### Within each story

- Tests are written and confirmed to FAIL before the implementation tasks they cover.
- Models / SQL helpers (T026–T029) before service-level functions (T028).
- Service functions before HTTP handlers (T030–T032).
- Backend before frontend wiring of the same capability.

### Parallel opportunities

- All `[P]` Setup tasks run in parallel.
- All `[P]` Foundational tasks run in parallel within their grouping (database vs. backend vs. frontend foundation).
- Once Phase 2 ships, US1, US2, US3 (all P1) can run in parallel on three developers — they touch disjoint files (`reservation/`, `inventory/`, `expirer/`).
- Within US1: T023, T024, T025 (tests) run in parallel; T026, T027 (helpers) run in parallel; T033, T034 (frontend) run in parallel.
- Within US4: T047, T048, T049 (tests) run in parallel.
- US6 is mostly self-contained; T058–T061 can interleave freely once US1 + US2 are merged.

---

## Parallel Example: User Story 1 (MVP)

```bash
# Step 1: Launch all three integration tests in parallel (must fail until implementation lands)
Task: "Concurrent reservation integration test in backend/tests/integration/reservation_concurrency_test.go"
Task: "Idempotency replay integration test in backend/tests/integration/idempotency_replay_test.go"
Task: "Idempotency concurrency test in backend/tests/integration/idempotency_concurrency_test.go"

# Step 2: Launch the two pure helpers in parallel
Task: "Hash helper in backend/internal/reservation/hash.go"
Task: "Types in backend/internal/reservation/types.go"

# Step 3: Backend services and handlers (sequential within the same files)
Task: "create.go (the FOR UPDATE transaction)"
Task: "idempotency.go (claim-or-observe pattern)"
Task: "POST /api/sales handler"
Task: "POST /api/sales/{id}/reservations handler + idempotency wiring"

# Step 4: Frontend in parallel
Task: "useReservation hook"
Task: "ReserveForm component"
```

---

## Implementation Strategy

### MVP first (User Story 1 only)

1. Complete Phase 1 (Setup).
2. Complete Phase 2 (Foundational). **Critical — blocks every user story.**
3. Complete Phase 3 (US1 — atomic reservation + idempotency).
4. **STOP and VALIDATE**: Run T023, T024, T025. All must pass. The no-oversell invariant is now demonstrable in isolation.
5. Optionally run a manual variant of the load test by writing a quick `go run` script that hits the running server; this is a preview of US6.

### Incremental delivery

1. Setup + Foundational + US1 → ship as "MVP: provably atomic reservation."
2. + US2 → ship as "Live dashboard."
3. + US3 → ship as "Reservations expire on a deterministic schedule" (the flash-sale dynamic).
4. + US4 → ship as "Users can release early; desync handled gracefully."
5. + US5 → ship as "Production-grade UX for all conflict states."
6. + US6 → ship as "CI-gated proof of no oversells under load."
7. + Polish → ship as v1.0.

### Constitutional gates per phase

Before merging any story's tasks, confirm:

- **I. Consistency**: New code paths fail closed on lock timeout / DB unavailability. No retries that mask inconsistency.
- **II. Pessimistic locking**: New mutation paths use `SELECT ... FOR UPDATE`. No CAS, no version columns.
- **III. Normalized schema**: No new denormalized counters.
- **IV. Stack discipline**: No new languages, frameworks, or routers introduced.
- **V. Concurrency tests**: New inventory-mutating code paths have at least one integration test that drives concurrent goroutines against the real Docker Postgres.

---

## Notes

- `[P]` tasks touch different files and have no incomplete prerequisites among the tasks above them.
- Each user story is independently demonstrable and ships value on its own.
- Tests are mandatory because Constitution Principle V requires them for inventory-mutating code paths; do not skip them.
- Commit after each task or logical group; the spec-kit `after_tasks` hook is available for auto-commits.
- The MVP is User Story 1 alone — once T023/T024/T025 pass, the headline guarantee is real.
- File paths are exact. Any LLM continuing this work should be able to execute each task without re-deriving the project layout.
