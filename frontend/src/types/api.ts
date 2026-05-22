// Wire-level types mirroring backend contracts at
// specs/001-flash-sale-reservation/contracts/api.md.

export type Inventory = {
  sale_id: string;
  total_capacity: number;
  currently_reserved: number;
  available_to_reserve: number;
};

export type ReservationStatus = "active" | "released" | "expired";

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

export type Sale = {
  id: string;
  name: string;
  total_capacity: number;
  created_at: string;
};

export type ApiErrorCode =
  | "INVALID_QUANTITY"
  | "SALE_NOT_FOUND"
  | "RESERVATION_NOT_FOUND"
  | "RESERVATION_TERMINAL"
  | "INSUFFICIENT_STOCK"
  | "IDEMPOTENCY_KEY_MISMATCH"
  | "LOCK_TIMEOUT"
  | "VALIDATION"
  | "INTERNAL";

export type ApiErrorBody = {
  code: ApiErrorCode;
  message: string;
  details?: Record<string, unknown>;
};

export type ApiErrorEnvelope = {
  error: ApiErrorBody;
};

// Idempotent terminal-replay response shape from DELETE /reservations/:id.
export type ReleaseResponse =
  | (Reservation & { code?: never })
  | { id: string; status: "expired" | "released"; code: "RESERVATION_TERMINAL" };
