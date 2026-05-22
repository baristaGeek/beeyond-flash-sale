import { SalePage } from "./pages/SalePage";

// The demo seed (`make seed`) inserts a sale with this deterministic UUID,
// so the frontend has something to render even without a `?sale=...` query
// param. Kept in sync with backend/cmd/seed/main.go DemoSaleID.
const DEMO_SALE_ID = "00000001-0000-4000-8000-000000000000";

function readSaleIdFromUrl(): string {
  if (typeof window === "undefined") return DEMO_SALE_ID;
  const params = new URLSearchParams(window.location.search);
  return params.get("sale") ?? DEMO_SALE_ID;
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
      <SalePage saleId={saleId} />
    </main>
  );
}
