# API Contract: Flash Sale Inventory Reservation System

**Date**: 2026-05-22
**Spec**: [../spec.md](../spec.md)
**Data model**: [../data-model.md](../data-model.md)

A small REST API over JSON. Every mutating endpoint opens a transaction at the handler
entry and commits or rolls back before responding (constitutional requirement). Every
error is a typed code in addition to the HTTP status, so the frontend renders states by
code, not by status.

---

## Conventions

- **Base path**: `/api`
- **Content type**: `application/json` (request and response).
- **Session identification**: Client sends a stable `X-Session-Id` header (UUID,
  generated and persisted by the React app in `localStorage` on first visit). Anonymous
  but stable per browser; no server-side user accounts in v1.
- **Time**: All timestamps are RFC 3339 / ISO 8601 UTC, e.g. `2026-05-22T14:30:00Z`.
- **IDs**: UUIDv4.

---

## Error envelope

Every non-2xx response uses the same envelope:

```json
{
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Only 3 units are currently available.",
    "details": { "available_to_reserve": 3 }
  }
}
```

### Error codes

| Code | HTTP | Meaning | UI mapping |
|------|------|---------|------------|
| `INVALID_QUANTITY` | 400 | Quantity is missing, ≤ 0, or non-integer. | Form-validation message. |
| `SALE_NOT_FOUND` | 404 | No sale exists with that ID. | Neutral "sale unavailable" state. |
| `RESERVATION_NOT_FOUND` | 404 | No active reservation matches the ID + session. | Neutral "no longer active" state. |
| `RESERVATION_TERMINAL` | 200 | Release request hit a reservation already released or expired. Treated as success (idempotent) — included in the table so the frontend can show a calm "already released" toast. | Calm informational message. |
| `INSUFFICIENT_STOCK` | 409 | Requested quantity exceeds Available. `details.available_to_reserve` carries the current count. | "Not enough stock; X available — try a smaller quantity." |
| `IDEMPOTENCY_KEY_MISMATCH` | 422 | An `Idempotency-Key` was reused with a request body that does not match the body of the original request that first used the key. `details.original_request_hash` and `details.submitted_request_hash` are included for debugging. | "This request conflicts with an earlier one we already processed — please start a new attempt." |
| `LOCK_TIMEOUT` | 503 | Row-level lock could not be acquired within the configured timeout. | "System busy, please try again" with retry. |
| `VALIDATION` | 400 | Generic input validation (malformed JSON, bad UUID). | Form-validation message. |
| `INTERNAL` | 500 | Unhandled error. | Generic "something went wrong" with retry. |

Note `RESERVATION_TERMINAL` returns HTTP 200 (not an error status) because per FR-014 it
is an idempotent success. It still appears in the table because the response body carries
this code so the UI can render the "already released" state distinctly from a true success.

---

## Endpoints

### 1. Create a sale (admin / seed)

Used by the load harness and dev workflows. Not exposed in the customer UI.

```
POST /api/sales
```

**Request**

```json
{
  "name": "Friday Drop",
  "total_capacity": 100
}
```

**Response 201**

```json
{
  "id": "5b1e8c3a-...",
  "name": "Friday Drop",
  "total_capacity": 100,
  "created_at": "2026-05-22T14:30:00Z"
}
```

**Errors**: `VALIDATION`, `INTERNAL`.

---

### 2a. List inventory (multi-product dashboard)

```
GET /api/sales
```

**Response 200**

```json
{
  "sales": [
    {
      "sale_id": "00000001-0000-4000-8000-000000000001",
      "name": "Vintage Camera",
      "total_capacity": 20,
      "currently_reserved": 3,
      "available_to_reserve": 17
    },
    {
      "sale_id": "00000001-0000-4000-8000-000000000002",
      "name": "Mechanical Watch",
      "total_capacity": 10,
      "currently_reserved": 0,
      "available_to_reserve": 10
    }
  ]
}
```

**Errors**: `INTERNAL`.

**Notes**:
- One round-trip for the entire catalog so a multi-product dashboard does not need N+1 polls.
- Sorted by `created_at` ascending for stable display order across polls.
- No transactional lock — best-effort reads with the same ≤1s freshness budget as endpoint 2b.
- Invariant `total_capacity = currently_reserved + available_to_reserve` holds per row.

---

### 2b. Read inventory state (single sale)

```
GET /api/sales/{sale_id}/inventory
```

**Response 200**

```json
{
  "sale_id": "5b1e8c3a-...",
  "name": "Vintage Camera",
  "total_capacity": 100,
  "currently_reserved": 12,
  "available_to_reserve": 88
}
```

**Errors**: `SALE_NOT_FOUND`, `INTERNAL`.

**Notes**:
- No transactional lock — best-effort read at ≤1s freshness budget.
- Invariant `total_capacity = currently_reserved + available_to_reserve` holds at every
  observed quiescent point.

---

### 3. Create a reservation (the consistency-critical endpoint)

```
POST /api/sales/{sale_id}/reservations
Headers:
  X-Session-Id:    <uuid>       (required)
  Idempotency-Key: <string>     (required; 1–128 chars)
```

**Request**

```json
{ "quantity": 3 }
```

**Response 201**

```json
{
  "id": "9d4a...",
  "sale_id": "5b1e8c3a-...",
  "quantity": 3,
  "status": "active",
  "created_at": "2026-05-22T14:30:00Z",
  "expires_at": "2026-05-22T14:31:00Z"
}
```

**Errors**: `VALIDATION` (missing/empty `Idempotency-Key`), `INVALID_QUANTITY`,
`SALE_NOT_FOUND`, `INSUFFICIENT_STOCK` (with `details.available_to_reserve`),
`IDEMPOTENCY_KEY_MISMATCH`, `LOCK_TIMEOUT`, `INTERNAL`.

**Server flow**: Opens a transaction with `SET LOCAL lock_timeout`, applies the
idempotency claim-or-observe pattern wrapping the consistency-critical insert.
See `data-model.md § reservations.create with Idempotency-Key` for the exact SQL.
Specifically:

1. **First request with this key** (scope: `session_id` + `idempotency_key`):
   the request is processed normally; the final HTTP status code and response body are
   persisted alongside a SHA-256 hash of the canonical request body before commit. The
   reservation insert and the idempotency record insert succeed or fail together in one
   transaction.
2. **Replay with the same key and a matching request body**: the server returns the
   *exact same response* as the first request — same `id`, same status code, same body.
   Stock is **not** decremented again. This is true whether the original request
   succeeded (`201`) or was rejected (`409 INSUFFICIENT_STOCK`); replays of rejections
   are also idempotent.
3. **Replay with the same key and a *different* request body**: the server returns
   `422 IDEMPOTENCY_KEY_MISMATCH`. Stock is not touched. The client is expected to
   resolve the conflict by either retrying with the original payload or starting a fresh
   attempt under a new key.
4. **Concurrent requests with the same key**: serialized at the database via a row-level
   lock on the idempotency record; the loser blocks until the winner commits, then reads
   the cached response and returns it (or returns `IDEMPOTENCY_KEY_MISMATCH` if its
   payload differs).

The request body that is hashed for comparison is the canonical JSON form of `{ "quantity": N }`
plus the path parameter `sale_id` — i.e., two requests that target different sales or
different quantities are *not* the same logical request even if they share an
`Idempotency-Key`.

**Idempotency key lifetime**: 24 hours from creation. Replays after the record has been
garbage-collected behave as a fresh request (no replay protection); clients SHOULD treat
the key as single-use across that window in practice.

---

### 4. Release a reservation (manual)

```
DELETE /api/reservations/{reservation_id}
Headers: X-Session-Id: <uuid>
```

**Response 200** — successful release of an active reservation:

```json
{
  "id": "9d4a...",
  "status": "released",
  "released_at": "2026-05-22T14:30:42Z"
}
```

**Response 200** — already-terminal (idempotent path, includes `RESERVATION_TERMINAL` code):

```json
{
  "id": "9d4a...",
  "status": "expired",
  "code": "RESERVATION_TERMINAL"
}
```

**Errors**: `RESERVATION_NOT_FOUND` (404 — also returned when the reservation exists but
belongs to a different session, to avoid leaking existence), `LOCK_TIMEOUT`, `INTERNAL`.

---

### 5. Read a reservation (optional, for timer resume / debugging)

```
GET /api/reservations/{reservation_id}
Headers: X-Session-Id: <uuid>
```

**Response 200**

```json
{
  "id": "9d4a...",
  "sale_id": "5b1e8c3a-...",
  "quantity": 3,
  "status": "active",
  "created_at": "2026-05-22T14:30:00Z",
  "expires_at": "2026-05-22T14:31:00Z",
  "released_at": null,
  "expired_at": null
}
```

**Errors**: `RESERVATION_NOT_FOUND`, `INTERNAL`.

**Notes**: Enables the frontend to re-synchronize its countdown timer using server-side
`expires_at` (which is the authoritative TTL clock per the spec's assumptions).

---

## TypeScript types (frontend mirror)

```ts
export type Inventory = {
  sale_id: string;
  name: string;
  total_capacity: number;
  currently_reserved: number;
  available_to_reserve: number;
};

export type InventoryListResponse = {
  sales: Inventory[];
};

export type ReservationStatus = 'active' | 'released' | 'expired';

export type Reservation = {
  id: string;
  sale_id: string;
  quantity: number;
  status: ReservationStatus;
  created_at: string;
  expires_at: string;
  released_at: string | null;
  expired_at: string | null;
};

export type ApiErrorCode =
  | 'INVALID_QUANTITY'
  | 'SALE_NOT_FOUND'
  | 'RESERVATION_NOT_FOUND'
  | 'RESERVATION_TERMINAL'
  | 'INSUFFICIENT_STOCK'
  | 'IDEMPOTENCY_KEY_MISMATCH'
  | 'LOCK_TIMEOUT'
  | 'VALIDATION'
  | 'INTERNAL';

export type ApiError = {
  error: {
    code: ApiErrorCode;
    message: string;
    details?: Record<string, unknown>;
  };
};
```

The discriminated union of `ApiErrorCode` is what the React UI switches on to render
distinct visual states (FR-016, FR-017).
