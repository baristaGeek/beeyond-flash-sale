// Wire-level types mirroring backend contracts at
// specs/001-flash-sale-reservation/contracts/api.md.

export type InventoryRow = {
  sale_id: string;
  name: string;
  total_capacity: number;
  currently_reserved: number;
  available_to_reserve: number;
};

export type InventoryListResponse = {
  sales: InventoryRow[];
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

// DELETE /api/reservations/{id} can return either an updated Reservation
// (happy path) or a terminal shape carrying a RESERVATION_TERMINAL code
// (idempotent path). The discriminator is the optional `code` field.
export type ReleaseTerminal = {
  id: string;
  status: "expired" | "released";
  code: "RESERVATION_TERMINAL";
  message: string;
};

export type ReleaseResponse = Reservation | ReleaseTerminal;

export function isReleaseTerminal(r: ReleaseResponse): r is ReleaseTerminal {
  return (r as ReleaseTerminal).code === "RESERVATION_TERMINAL";
}
