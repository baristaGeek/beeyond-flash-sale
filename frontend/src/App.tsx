import { SalePage } from "./pages/SalePage";

// For v1 the demo runs a single sale. The sale id can be supplied via URL
// query param `?sale=<uuid>` so the load-test harness's seeded sale can be
// viewed without redeploying.
function readSaleIdFromUrl(): string | null {
  if (typeof window === "undefined") return null;
  const params = new URLSearchParams(window.location.search);
  return params.get("sale");
}

export default function App() {
  const saleId = readSaleIdFromUrl();
  return (
    <main style={{ maxWidth: 720, margin: "32px auto", padding: "0 16px", fontFamily: "system-ui, sans-serif" }}>
      <header style={{ marginBottom: 24 }}>
        <h1 style={{ margin: 0 }}>Flash Sale</h1>
        <p style={{ marginTop: 4, color: "#555" }}>
          Reservations expire 60 seconds after they're placed.
        </p>
      </header>
      {saleId ? (
        <SalePage saleId={saleId} />
      ) : (
        <p>
          Provide a sale id via <code>?sale=&lt;uuid&gt;</code> in the URL. Seed one with{" "}
          <code>POST /api/sales</code>.
        </p>
      )}
    </main>
  );
}
