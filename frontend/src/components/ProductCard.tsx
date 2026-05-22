import { useState } from "react";
import { StockMeter } from "./StockMeter";
import type { InventoryRow, Reservation } from "@/types/api";
import { reserveSale, ApiError } from "@/api/client";
import { useToast } from "@/hooks/useToast";

type Props = {
  row: InventoryRow;
  onReserved: (reservation: Reservation) => void;
};

export function ProductCard({ row, onReserved }: Props) {
  const [isPending, setIsPending] = useState(false);
  const { showToast } = useToast();
  const outOfStock = row.available_to_reserve <= 0;

  const initial = row.name.charAt(0).toUpperCase();

  const handleReserve = async () => {
    if (outOfStock || isPending) return;
    setIsPending(true);
    try {
      const reservation = await reserveSale(row.sale_id, 1, crypto.randomUUID());
      onReserved(reservation);
    } catch (err) {
      if (err instanceof ApiError && err.body.code === "INSUFFICIENT_STOCK") {
        showToast({
          title: "Item Taken",
          body: `Sorry, the ${row.name} was just reserved by another user.`,
        });
      } else if (err instanceof ApiError && err.body.code === "LOCK_TIMEOUT") {
        showToast({
          title: "System Busy",
          body: "Please try again in a moment.",
        });
      } else if (err instanceof ApiError) {
        showToast({ title: "Could not reserve", body: err.body.message });
      } else {
        showToast({
          title: "Network error",
          body: "Could not reach the server. Please retry.",
        });
      }
    } finally {
      setIsPending(false);
    }
  };

  return (
    <article className="product-card">
      <div className="product-card__header">
        <div className="product-avatar" aria-hidden="true">{initial}</div>
        <h3 className="product-name">{row.name}</h3>
      </div>
      <StockMeter total={row.total_capacity} available={row.available_to_reserve} />
      <button
        type="button"
        className="btn"
        disabled={outOfStock || isPending}
        onClick={handleReserve}
      >
        {outOfStock ? "Out of Stock" : isPending ? "Reserving…" : "Reserve Item"}
      </button>
    </article>
  );
}
