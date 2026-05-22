import { useCallback, useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Header } from "@/components/Header";
import { ProductCard } from "@/components/ProductCard";
import { ReservationsPanel } from "@/components/ReservationsPanel";
import { SALES_QUERY_KEY, useSalesList } from "@/hooks/useSalesList";
import { useReservationsTracker } from "@/hooks/useReservationsTracker";

export function InventoryPage() {
  const queryClient = useQueryClient();
  const sales = useSalesList();
  const { reservations, trackReservation, dropReservation } = useReservationsTracker();

  const productLookup = useMemo(() => {
    const map = new Map<string, NonNullable<typeof sales.data>["sales"][number]>();
    sales.data?.sales.forEach((s) => map.set(s.sale_id, s));
    return map;
  }, [sales.data]);

  const handleRefresh = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: SALES_QUERY_KEY });
  }, [queryClient]);

  const isLive = sales.isSuccess && !sales.isError;

  return (
    <>
      <Header isLive={isLive} onRefresh={handleRefresh} isRefreshing={sales.isFetching} />
      <div className="page-shell">
        <section>
          <h2 className="section-title">Available Inventory</h2>
          {sales.isLoading && <p>Loading inventory…</p>}
          {sales.isError && <p>Could not load inventory. Will retry…</p>}
          {sales.data && sales.data.sales.length === 0 && (
            <p>No sales seeded yet. Run <code>make seed</code> to populate the demo catalog.</p>
          )}
          {sales.data && (
            <div className="inventory-grid">
              {sales.data.sales.map((row) => (
                <ProductCard
                  key={row.sale_id}
                  row={row}
                  onReserved={(r) => {
                    trackReservation(r);
                    queryClient.invalidateQueries({ queryKey: SALES_QUERY_KEY });
                  }}
                />
              ))}
            </div>
          )}
        </section>
        <ReservationsPanel
          reservations={reservations}
          productLookup={productLookup}
          onTerminal={(id) => {
            dropReservation(id);
            queryClient.invalidateQueries({ queryKey: SALES_QUERY_KEY });
          }}
        />
      </div>
    </>
  );
}
