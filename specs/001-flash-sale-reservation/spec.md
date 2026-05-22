# Feature Specification: Flash Sale Inventory Reservation System

**Feature Branch**: `001-flash-sale-reservation`

**Created**: 2026-05-22

**Status**: Draft

**Input**: User description: "High-concurrency Flash Sale Inventory Reservation System with real-time
dashboard, atomic N-unit reservations under 100+ concurrent load, strict 60-second TTL with
deterministic expiry, manual release with desync handling, UI-mapped conflict states, and
load-testing capabilities to prove atomic constraints."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Atomic Reservation Under Contention (Priority: P1)

A shopper viewing the flash sale page sees how many units are still available. They choose a
quantity (N units) and submit a reservation request. The system either grants the full N-unit
reservation and decrements the Available count by exactly N, or rejects the request because
fewer than N units are Available. Even when many other shoppers are racing to reserve the same
stock at the exact same moment, the system never allows the total reserved across all shoppers
to exceed the Total Capacity.

**Why this priority**: This is the core promise of the system. Without an atomic, no-oversell
reservation flow, none of the other capabilities matter — the entire value of a flash sale is
the credible guarantee that "reserved" means a held seat that no one else can take.

**Independent Test**: Seed a sale with a small finite capacity (e.g., 10 units). Run 100
concurrent reservation requests, each asking for 1 unit. Verify exactly 10 reservations succeed
and 90 are rejected with a clear "insufficient stock" response. Total reserved units must
equal 10, not 11, not 9.

**Acceptance Scenarios**:

1. **Given** a sale with Available = 10, **When** a single user requests to reserve 3 units,
   **Then** the request succeeds and Available becomes 7.
2. **Given** a sale with Available = 1 and two users submitting reservations for 1 unit each
   at the same instant, **When** both requests are processed, **Then** exactly one succeeds
   and one fails with an insufficient-stock conflict.
3. **Given** a sale with Available = 0, **When** any user requests any positive quantity,
   **Then** the request is rejected with an insufficient-stock conflict and Available stays at 0.
4. **Given** a sale with Available = 5, **When** a user requests to reserve 7 units,
   **Then** the request is rejected (the system never grants a partial 5-unit reservation
   when 7 were requested).
5. **Given** 100 concurrent users each requesting 1 unit against Available = 10,
   **When** all requests resolve, **Then** the count of granted reservations is exactly 10
   and Available is exactly 0.

---

### User Story 2 - Live Inventory Dashboard (Priority: P1)

Anyone viewing the flash sale page can see, in real time, three explicit numbers: the Total
Capacity of the sale, how many units are Currently Reserved (held by active reservations), and
how many are Available to Reserve. The numbers update as other shoppers reserve, release, or
let their holds expire.

**Why this priority**: The dashboard is the only way users see the state they're acting on.
Without it, users cannot make informed decisions and conflict errors feel arbitrary. It is also
the primary surface for verifying system correctness during a demo or audit.

**Independent Test**: With the reservation endpoint stubbed or seeded directly in the database,
open the dashboard in a browser and verify Total / Reserved / Available are visible, labeled
clearly, and update within a short bounded interval (≤1 second) when underlying state changes.

**Acceptance Scenarios**:

1. **Given** a sale with Total = 100 and no active reservations, **When** the page loads,
   **Then** the dashboard shows Total = 100, Reserved = 0, Available = 100.
2. **Given** the dashboard is open and Available = 50, **When** another user reserves 10 units
   in a separate session, **Then** the dashboard reflects Reserved = 10 and Available = 40
   within 1 second.
3. **Given** the dashboard is open and Reserved = 10, **When** the 60-second TTL expires on
   those reservations, **Then** the dashboard reflects Reserved = 0 and Available returns to
   the pre-reservation total within 1 second of expiry.
4. **Given** any displayed state, **Then** the invariant Total = Reserved + Available is
   visibly maintained.

---

### User Story 3 - Deterministic 60-Second TTL Expiry (Priority: P1)

When a shopper reserves units but does not finalize within 60 seconds, the reservation
automatically expires. The held units are returned to the Available pool, and the reservation
record is tombstoned (kept permanently for audit, but no longer counted as active stock). The
expiry is deterministic — it happens at the 60-second mark for that reservation, not on the
next user action or page refresh.

**Why this priority**: A flash sale collapses if held inventory can be hoarded indefinitely.
Deterministic TTL is what makes the sale "flash" — units that aren't acted on quickly
flow back into the pool for other shoppers.

**Independent Test**: Create a reservation, do nothing, and verify that exactly at the
60-second mark (within a small tolerance), the reservation is no longer active, the
Available count has returned to its pre-reservation value, and the reservation record is
present in storage with a terminal status.

**Acceptance Scenarios**:

1. **Given** a reservation created at time T with quantity 3 against Available = 10,
   **When** time T + 60s is reached without any action on the reservation,
   **Then** the reservation is marked expired and Available returns to 10.
2. **Given** an expired reservation, **When** any subsequent action references it,
   **Then** the system recognizes it as terminal (not active) and does not return its units
   to the Available pool a second time.
3. **Given** 50 reservations created at staggered times, **When** time advances past each
   reservation's 60-second mark, **Then** each individually expires at its own T+60s, not
   in batches tied to other events.

---

### User Story 4 - Manual Release With Desync Handling (Priority: P2)

A shopper changes their mind and clicks "Release" on their active reservation. The held units
return to the Available pool immediately and the reservation is tombstoned. If, however, the
shopper's UI timer has drifted out of sync with the backend and the reservation has already
auto-expired by the time the release request arrives, the system responds gracefully: it does
not error loudly, it does not return the units a second time, and the UI shows a calm
"already released" state rather than a red error.

**Why this priority**: Manual release is a courtesy feature, not the core no-oversell guarantee,
but the desync edge case is a credible failure mode that customers will hit. Mishandling it
either double-returns stock (a correctness bug) or shows a scary error (a trust bug).

**Independent Test**: (a) Create a reservation, release it before 60s — verify units return
exactly once. (b) Create a reservation, wait past 60s for auto-expiry, then submit a release
request — verify the response is a graceful "no-op" and the Available count is unchanged from
its post-expiry value.

**Acceptance Scenarios**:

1. **Given** an active reservation of 4 units, **When** the user submits a release before
   T+60s, **Then** Available increases by 4 and the reservation is marked released.
2. **Given** a reservation that has already expired via TTL, **When** the user submits a
   release request, **Then** the system responds with a non-error "already terminal" status
   and the Available count remains unchanged.
3. **Given** a reservation that has already been released, **When** a duplicate release
   request arrives (e.g., from a retried button click), **Then** the system responds with
   the same non-error "already terminal" status and does not double-return stock.

---

### User Story 5 - Graceful Conflict Mapping in the UI (Priority: P2)

When the backend rejects a reservation request (most commonly because stock has been exhausted
mid-flight), the UI translates the backend's machine-readable error into a friendly,
actionable visual state — e.g., "Sold out, please try a smaller quantity" or "Stock changed
while you were deciding; here's the new available count." Users never see a raw HTTP status
or a stack trace.

**Why this priority**: In a flash sale, conflicts are not exceptional — they are the expected
outcome for most users. The product is only usable if conflicts read as normal flow events,
not as system failures.

**Independent Test**: Force each conflict type from the backend (insufficient stock, expired
reservation, reservation not found, validation failure) and verify the UI renders a distinct,
user-friendly state for each — never a raw error code, never a stack trace.

**Acceptance Scenarios**:

1. **Given** a user submits a reservation request that exceeds Available stock, **When** the
   backend returns the conflict, **Then** the UI shows a non-alarming "not enough stock"
   message that includes the current Available count and offers a retry.
2. **Given** a user submits a release for a reservation that does not exist,
   **When** the backend returns "not found", **Then** the UI shows a neutral
   "this reservation is no longer active" state.
3. **Given** any backend error, **When** rendered in the UI, **Then** no raw HTTP status
   code, JSON body, or stack trace appears in the user-visible surface.

---

### User Story 6 - Provable Concurrency Correctness via Load Testing (Priority: P3)

An engineer runs a single command that drives ≥100 concurrent reservation requests against a
seeded sale and produces a report. The report shows the total requested units, the total
granted units, the final database state, and an explicit pass/fail line: "no-oversell invariant
HOLDS" or "VIOLATED with X excess units." The same command run twice on the same seed produces
the same report.

**Why this priority**: This is the harness that turns the no-oversell claim from an assertion
into a demonstrable, reproducible property. It is a precondition for trusting any deployment
of the feature, but it is not user-facing and can ship after the core capability.

**Independent Test**: Seed a sale with capacity = 10. Run the load test asking for 100
single-unit reservations in parallel. Verify the report shows exactly 10 granted, 90 rejected,
final Reserved = 10, final Available = 0, and prints "no-oversell invariant HOLDS." Re-run on
the same seed and verify the report is identical.

**Acceptance Scenarios**:

1. **Given** a sale seeded to Total = 10, **When** the load test fires 100 parallel single-unit
   reservation requests, **Then** the report shows exactly 10 successes, 90 conflicts, and
   prints "no-oversell invariant HOLDS."
2. **Given** the same seed and the same load profile, **When** the load test is run twice,
   **Then** the two reports are equivalent (same outcome lines, modulo timestamps).
3. **Given** any pass/fail outcome, **When** the report is reviewed, **Then** it contains
   enough state (timestamps, request counts, final inventory counts) to reproduce the run.

---

### Edge Cases

- **Reservation quantity is zero or negative**: System MUST reject with a validation error,
  not silently no-op.
- **Reservation quantity exceeds Total Capacity**: System MUST reject without grant; this is
  the same conflict surface as "insufficient available stock."
- **Reservation created when Available is exactly equal to requested quantity**: System MUST
  grant and the new Available MUST be exactly 0 — boundary, not off-by-one.
- **Simultaneous arrival of two requests that together exceed Available**: System MUST grant
  the first to acquire the lock and reject the rest, never partially grant both.
- **Release request for a non-existent reservation ID**: System MUST respond with a neutral
  "no such active reservation" outcome — same handling as already-expired.
- **TTL job processes an already-released reservation**: System MUST treat it as a no-op;
  release wins, no double-return.
- **Manual release lands during the TTL job's expiry transaction**: System MUST serialize the
  two paths so that exactly one of them returns the stock and the other observes a terminal
  state.
- **Clock drift between client and server**: System MUST use server time as the authority for
  the 60-second TTL; client-side timers are presentation only.
- **Page refresh during an active reservation**: User's active reservation MUST remain
  retrievable and the timer MUST resume from the server's known expiry time.
- **Server restart while reservations are active**: All active reservations MUST survive the
  restart and continue to expire at their original T+60s times.
- **User attempts to hold more than one reservation per session**: The system's behavior on
  this is documented in Assumptions (default: allowed; not enforced as a hard limit in v1).

## Requirements *(mandatory)*

### Functional Requirements

**Inventory state & dashboard**

- **FR-001**: System MUST persist a Total Capacity per sale that is set at sale creation and
  does not change during the sale.
- **FR-002**: System MUST expose the current Total, Currently Reserved, and Available to
  Reserve counts via a read endpoint suitable for dashboard consumption.
- **FR-003**: System MUST maintain the invariant Total = Currently Reserved + Available to
  Reserve at every quiescent point (i.e., between transactions).
- **FR-004**: System MUST update the dashboard view of inventory state within 1 second of the
  underlying state change.

**Reservation creation**

- **FR-005**: Users MUST be able to request a reservation of N units against a sale, where N
  is a positive integer.
- **FR-006**: System MUST reject any reservation request with N ≤ 0 as a validation error.
- **FR-007**: System MUST atomically grant or deny a reservation request such that no
  combination of concurrent grants can produce a Currently Reserved value exceeding Total.
- **FR-008**: System MUST reject reservation requests for which N exceeds the current
  Available count, returning a typed "insufficient stock" conflict that includes the
  current Available count.
- **FR-009**: On successful reservation, System MUST record the reservation with: a unique
  identifier, owning user/session identifier, sale identifier, quantity, creation timestamp,
  expiry timestamp (creation + 60s), and status = "active."

**TTL expiry**

- **FR-010**: System MUST cause every active reservation to transition to a terminal
  "expired" status exactly 60 seconds after its creation timestamp (server time), within a
  small bounded tolerance (≤1 second).
- **FR-011**: On expiry, System MUST return the reservation's units to the Available count
  exactly once, regardless of how many times the expiry path is triggered.
- **FR-012**: System MUST retain expired reservations as tombstones (read-only audit records)
  rather than deleting them.

**Manual release**

- **FR-013**: Users MUST be able to release their own active reservation before its expiry,
  returning its units to the Available count.
- **FR-014**: System MUST treat a release request against an already-terminal reservation
  (expired, released, or unknown) as a non-error, idempotent outcome that does not modify
  the inventory.
- **FR-015**: System MUST serialize concurrent release and TTL-expiry paths on the same
  reservation so that exactly one path returns the stock.

**Error mapping & UI states**

- **FR-016**: System MUST surface every rejection as a typed, machine-readable error code
  (e.g., INSUFFICIENT_STOCK, INVALID_QUANTITY, RESERVATION_TERMINAL, RESERVATION_NOT_FOUND)
  in addition to a human-readable message.
- **FR-017**: UI MUST map each error code to a distinct, user-friendly visual state and MUST
  NOT display raw error codes, HTTP statuses, or stack traces to users.

**Traceability**

- **FR-018**: System MUST record the timeline of every reservation (created, released,
  expired) with timestamps such that any reservation's full history can be reconstructed
  from storage alone.
- **FR-019**: System MUST identify each reservation with a stable, unique identifier visible
  to the owning user and usable as a support reference.

**Load testing & concurrency proof**

- **FR-020**: Project MUST include a load-testing harness that can issue ≥100 concurrent
  reservation requests against a configurable seeded sale.
- **FR-021**: Load-testing harness MUST produce a deterministic report containing: total
  requests issued, count of successes, count of each conflict type, final Reserved and
  Available counts, and an explicit pass/fail of the no-oversell invariant.
- **FR-022**: Load-testing harness MUST be runnable repeatedly against the same seed and
  produce equivalent reports (same pass/fail, same counts) across runs.

**Idempotent reservation requests**

- **FR-023**: System MUST require a client-supplied idempotency key on every
  reservation-create request. Requests that omit the key MUST be rejected with a
  typed validation error before any inventory state is touched.
- **FR-024**: When two reservation-create requests arrive with the same idempotency key
  and an identical request payload (same sale and same quantity), the system MUST
  return the same outcome — the same reservation identifier (when the original succeeded)
  or the same rejection — and MUST NOT decrement available stock more than once.
- **FR-025**: When two reservation-create requests arrive with the same idempotency key
  but a non-identical request payload, the system MUST reject the second request with a
  distinct, typed error (separate from insufficient-stock and from invalid-quantity)
  identifying the conflict as a key-payload mismatch, and MUST NOT decrement available
  stock.
- **FR-026**: Idempotency replay protection MUST persist for at least 24 hours from the
  original request; requests after that window MAY be treated as fresh requests.
- **FR-027**: Concurrent reservation requests sharing the same idempotency key MUST be
  serialized so that exactly one performs the underlying inventory change and the others
  observe its cached outcome (or the mismatch error, if their payloads differ).

### Key Entities

- **Sale**: The flash sale event. Attributes: identifier, total capacity (immutable for the
  sale's lifetime), human-readable name. Relationships: has many Reservations.
- **Reservation**: A user's hold on a quantity of units within a sale. Attributes: identifier,
  sale identifier, owner identifier (user/session), quantity, created-at timestamp, expires-at
  timestamp, status (active | released | expired). Relationships: belongs to one Sale.
- **Inventory View**: A derived read-model of a Sale showing Total / Currently Reserved /
  Available to Reserve. Not stored as denormalized counters; computed from Sale and active
  Reservations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001 (No-oversell, the headline metric)**: Across all load-test runs and all stress
  scenarios, the number of granted reservation units never exceeds the sale's Total Capacity.
  Target: 0 oversell events across ≥1,000 trial runs at varying contention levels.
- **SC-002 (Deterministic TTL)**: ≥99% of unreleased reservations transition to expired
  status within 60.0 seconds + 1 second of tolerance from their creation timestamp.
- **SC-003 (Dashboard freshness)**: After any state change, the dashboard reflects the new
  Total / Reserved / Available counts within 1 second for the active viewer.
- **SC-004 (Conflict UX)**: 100% of distinct backend rejection types map to a distinct,
  user-friendly UI state with no raw status codes or stack traces visible to users.
- **SC-005 (Reproducible load test)**: Two consecutive load-test runs against the same seed
  produce reports with identical success/conflict counts and identical pass/fail outcomes.
- **SC-006 (Graceful desync handling)**: A manual-release request submitted up to 30 seconds
  after the reservation's TTL has already fired produces a graceful, non-error UI state in
  100% of attempts.
- **SC-007 (Concurrency floor)**: System sustains at least 100 concurrent reservation
  requests against a single sale without violating SC-001 and without any request hanging
  indefinitely (every request resolves to success or a typed conflict within a bounded
  timeout).
- **SC-008 (Traceability)**: For any historical reservation identifier, an operator can
  reconstruct the full lifecycle (created → released/expired) from storage alone, including
  timestamps.

## Assumptions

- **Single sale at a time, multiple users**: The system targets one or a handful of active
  sales running simultaneously; the core test scenario is many users reserving against one
  sale, not many sales running in parallel.
- **No payment / checkout step in v1**: "Reservation" is the terminal active state. A
  reservation either expires at T+60s or is manually released; there is no separate
  "confirm purchase" flow that finalizes it into a permanent sale. If a future iteration
  adds checkout, the 60-second TTL becomes the window in which checkout must complete.
- **Anonymous users identified by session/token**: Users are identified by a stable
  session-level identifier (cookie or local token) rather than a full authenticated account.
  A user's reservation is releasable by anyone presenting that session identifier.
- **One sale, one inventory pool**: A Sale's Total Capacity is a single fungible pool; the
  system does not model variants (size, color) within a sale in v1.
- **No per-user reservation cap in v1**: A single session may hold multiple concurrent
  reservations against the same sale; a hard cap can be added later if abuse emerges.
- **Server time is authoritative**: All TTL math uses server clocks; client timers are for
  display only and may drift.
- **Dashboard freshness via polling is acceptable**: A short-interval poll (e.g., 1 second)
  satisfies the dashboard freshness requirement; a push channel (SSE/WebSocket) is a
  permitted but optional optimization.
- **Network and database availability assumed for normal operation**: The system's
  consistency-over-availability stance means that under datastore unavailability, the
  system will reject requests rather than degrade gracefully; this is by design, not a
  bug to be papered over.
