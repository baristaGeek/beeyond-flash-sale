import { ReservationCard } from "./ReservationCard";
import type { InventoryRow, Reservation } from "@/types/api";

type Props = {
  reservations: Reservation[];
  productLookup: Map<string, InventoryRow>;
  onTerminal: (reservationId: string) => void;
};

export function ReservationsPanel({ reservations, productLookup, onTerminal }: Props) {
  return (
    <aside className="reservations-panel">
      <h2 className="section-title">Your Reservations</h2>
      {reservations.length === 0 ? (
        <p className="reservations-empty">No active reservations yet. Reserve an item to see it here.</p>
      ) : (
        reservations.map((r) => (
          <ReservationCard
            key={r.id}
            reservation={r}
            productName={productLookup.get(r.sale_id)?.name ?? "Item"}
            onTerminal={onTerminal}
          />
        ))
      )}
    </aside>
  );
}
