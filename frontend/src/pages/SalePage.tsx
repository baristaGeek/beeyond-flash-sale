import { ReserveForm } from "@/components/ReserveForm";

type Props = {
  saleId: string;
};

// SalePage is the customer-facing surface for a single sale. In MVP scope
// (US1) it shows only the reservation form; the live inventory dashboard
// (US2) and the reservation timer (US3) will land in subsequent branches and
// render above and below the form respectively.
export function SalePage({ saleId }: Props) {
  return (
    <div>
      <div style={{ fontSize: 14, color: "#666", marginBottom: 8 }}>
        Sale: <code>{saleId}</code>
      </div>
      <ReserveForm saleId={saleId} />
    </div>
  );
}
