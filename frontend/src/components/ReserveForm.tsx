import { useState } from "react";
import { useReservation } from "@/hooks/useReservation";
import { describeError } from "@/api/errors";
import type { Reservation } from "@/types/api";

type Props = {
  saleId: string;
  onReserved?: (r: Reservation) => void;
};

export function ReserveForm({ saleId, onReserved }: Props) {
  const [quantity, setQuantity] = useState<number>(1);
  const reservation = useReservation(saleId);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!Number.isFinite(quantity) || quantity < 1) return;
    reservation.mutate(
      { quantity },
      {
        onSuccess: (r) => onReserved?.(r),
      },
    );
  };

  const handleNewAttempt = () => {
    reservation.resetIdempotencyKey();
    reservation.reset();
  };

  const err = reservation.error ? describeError(reservation.error) : null;

  return (
    <section
      style={{
        border: "1px solid #ddd",
        borderRadius: 8,
        padding: 16,
        marginTop: 16,
      }}
    >
      <h2 style={{ marginTop: 0 }}>Reserve units</h2>
      <form onSubmit={handleSubmit} style={{ display: "flex", gap: 12, alignItems: "center" }}>
        <label>
          Quantity:&nbsp;
          <input
            type="number"
            min={1}
            value={quantity}
            onChange={(e) => setQuantity(parseInt(e.target.value, 10))}
            disabled={reservation.isPending}
            style={{ width: 80, padding: 6 }}
          />
        </label>
        <button type="submit" disabled={reservation.isPending}>
          {reservation.isPending ? "Reserving…" : "Reserve"}
        </button>
      </form>

      {reservation.data && (
        <ReservedSummary reservation={reservation.data} onNewAttempt={handleNewAttempt} />
      )}

      {err && (
        <ConflictNotice
          code={err.code}
          message={err.body.message}
          details={err.body.details}
          onNewAttempt={handleNewAttempt}
        />
      )}
    </section>
  );
}

function ReservedSummary({
  reservation,
  onNewAttempt,
}: {
  reservation: Reservation;
  onNewAttempt: () => void;
}) {
  return (
    <div
      style={{
        marginTop: 16,
        padding: 12,
        background: "#e8f5e9",
        border: "1px solid #c8e6c9",
        borderRadius: 6,
      }}
    >
      <strong>Reserved {reservation.quantity} unit(s).</strong>
      <div style={{ fontSize: 12, color: "#444", marginTop: 4 }}>
        ID: <code>{reservation.id}</code>
      </div>
      <div style={{ fontSize: 12, color: "#444" }}>
        Expires at: {new Date(reservation.expires_at).toLocaleTimeString()}
      </div>
      <button onClick={onNewAttempt} style={{ marginTop: 8 }}>
        Make another reservation
      </button>
    </div>
  );
}

function ConflictNotice({
  code,
  message,
  details,
  onNewAttempt,
}: {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  onNewAttempt: () => void;
}) {
  const available =
    details && typeof details.available_to_reserve === "number"
      ? (details.available_to_reserve as number)
      : null;

  let headline = message;
  let suggestRetry = true;

  switch (code) {
    case "INSUFFICIENT_STOCK":
      headline =
        available != null
          ? `Not enough stock — only ${available} unit(s) available right now.`
          : message;
      break;
    case "IDEMPOTENCY_KEY_MISMATCH":
      headline =
        "This attempt key was already used for a different request. Please start a fresh attempt.";
      suggestRetry = true;
      break;
    case "LOCK_TIMEOUT":
      headline = "The system is busy. Please try again in a moment.";
      break;
    case "INVALID_QUANTITY":
      headline = "Quantity must be a positive whole number.";
      suggestRetry = false;
      break;
  }

  return (
    <div
      style={{
        marginTop: 16,
        padding: 12,
        background: "#fff3e0",
        border: "1px solid #ffe0b2",
        borderRadius: 6,
      }}
    >
      <strong>{headline}</strong>
      {suggestRetry && (
        <div style={{ marginTop: 8 }}>
          <button onClick={onNewAttempt}>Start a fresh attempt</button>
        </div>
      )}
    </div>
  );
}
