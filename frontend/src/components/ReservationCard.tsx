import { useState } from "react";
import { CountdownTimer } from "./CountdownTimer";
import type { Reservation } from "@/types/api";
import { isReleaseTerminal } from "@/types/api";
import { releaseReservation, ApiError } from "@/api/client";
import { useToast } from "@/hooks/useToast";

type Props = {
  reservation: Reservation;
  productName: string;
  onTerminal: (id: string) => void;
};

// Short reservation id badge used for display — last 4 chars of the UUID
// rendered as "#RES-XXXX".
function shortBadge(id: string): string {
  const tail = id.replace(/-/g, "").slice(-4).toUpperCase();
  return `#RES-${tail}`;
}

export function ReservationCard({ reservation, productName, onTerminal }: Props) {
  const [isReleasing, setIsReleasing] = useState(false);
  const { showToast } = useToast();

  const handleRelease = async () => {
    if (isReleasing) return;
    setIsReleasing(true);
    try {
      const r = await releaseReservation(reservation.id);
      // Whether actively released or already terminal — either way it's gone.
      if (isReleaseTerminal(r)) {
        showToast({
          title: "Already released",
          body: `${productName} was already released or expired.`,
          durationMs: 3500,
        });
      }
      onTerminal(reservation.id);
    } catch (err) {
      if (err instanceof ApiError && err.body.code === "RESERVATION_NOT_FOUND") {
        // Server doesn't know about it — drop from tracker.
        onTerminal(reservation.id);
      } else if (err instanceof ApiError) {
        showToast({ title: "Could not release", body: err.body.message });
      } else {
        showToast({
          title: "Network error",
          body: "Could not reach the server. Please retry.",
        });
      }
    } finally {
      setIsReleasing(false);
    }
  };

  return (
    <div className="reservation-card">
      <div className="reservation-card__row">
        <span className="reservation-card__name">{productName}</span>
        <span className="reservation-id-badge">{shortBadge(reservation.id)}</span>
      </div>
      <div className="countdown-label">Countdown</div>
      <CountdownTimer
        expiresAt={reservation.expires_at}
        onExpired={() => onTerminal(reservation.id)}
      />
      <div className="reservation-card__footer">
        <span className="unit-held">{reservation.quantity} Unit{reservation.quantity === 1 ? "" : "s"} Held</span>
        <button
          type="button"
          className="btn btn--small"
          onClick={handleRelease}
          disabled={isReleasing}
        >
          {isReleasing ? "Releasing…" : "Release"}
        </button>
      </div>
    </div>
  );
}
