import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { InventoryRow, Reservation } from "@/types/api";
import { ToastProvider } from "@/hooks/useToast";

// Hoisted mock for the API client. Vitest hoists vi.mock() calls above imports,
// so we capture the mock function via vi.hoisted to keep a stable reference
// that the test bodies can override per-case.
const { reserveSaleMock } = vi.hoisted(() => ({
  reserveSaleMock: vi.fn(),
}));

vi.mock("@/api/client", async () => {
  const actual = await vi.importActual<typeof import("@/api/client")>("@/api/client");
  return {
    ...actual,
    reserveSale: reserveSaleMock,
  };
});

// Import ProductCard AFTER the mock is registered so it picks up the mocked
// reserveSale.
import { ProductCard } from "./ProductCard";

const FAKE_ROW: InventoryRow = {
  sale_id: "00000001-0000-4000-8000-000000000001",
  name: "Vintage Camera",
  total_capacity: 20,
  currently_reserved: 3,
  available_to_reserve: 17,
};

const FAKE_RESERVATION: Reservation = {
  id: "9d4a1c2b-3e4f-5a6b-7c8d-9e0f1a2b3c4d",
  sale_id: FAKE_ROW.sale_id,
  quantity: 1,
  status: "active",
  created_at: "2026-05-22T14:30:00Z",
  expires_at: "2026-05-22T14:31:00Z",
  released_at: null,
  expired_at: null,
};

const IDEMPOTENCY_KEY = "test-idempotency-key";

function renderProductCard(
  row: InventoryRow,
  onReserved: (r: Reservation) => void = vi.fn(),
) {
  return render(
    <ToastProvider>
      <ProductCard row={row} onReserved={onReserved} />
    </ToastProvider>,
  );
}

describe("<ProductCard /> reserve-flow happy path", () => {
  beforeEach(() => {
    reserveSaleMock.mockReset();
    vi.spyOn(crypto, "randomUUID").mockReturnValue(
      IDEMPOTENCY_KEY as ReturnType<Crypto["randomUUID"]>,
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("calls reserveSale with sale_id, quantity=1, and a fresh idempotency key", async () => {
    reserveSaleMock.mockResolvedValueOnce(FAKE_RESERVATION);
    const onReserved = vi.fn();
    renderProductCard(FAKE_ROW, onReserved);

    await userEvent.click(screen.getByRole("button", { name: /reserve item/i }));

    await waitFor(() => {
      expect(reserveSaleMock).toHaveBeenCalledTimes(1);
    });
    expect(reserveSaleMock).toHaveBeenCalledWith(
      FAKE_ROW.sale_id,
      1,
      IDEMPOTENCY_KEY,
    );
  });

  it("invokes onReserved with the resolved Reservation on success", async () => {
    reserveSaleMock.mockResolvedValueOnce(FAKE_RESERVATION);
    const onReserved = vi.fn();
    renderProductCard(FAKE_ROW, onReserved);

    await userEvent.click(screen.getByRole("button", { name: /reserve item/i }));

    await waitFor(() => {
      expect(onReserved).toHaveBeenCalledTimes(1);
    });
    expect(onReserved).toHaveBeenCalledWith(FAKE_RESERVATION);
  });

  it("shows 'Reserving…' while the request is in flight, then returns to 'Reserve Item'", async () => {
    let resolveReserve!: (r: Reservation) => void;
    reserveSaleMock.mockImplementationOnce(
      () =>
        new Promise<Reservation>((resolve) => {
          resolveReserve = resolve;
        }),
    );
    renderProductCard(FAKE_ROW);

    await userEvent.click(screen.getByRole("button", { name: /reserve item/i }));

    const pendingButton = await screen.findByRole("button", { name: /reserving/i });
    expect(pendingButton).toBeDisabled();

    resolveReserve(FAKE_RESERVATION);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /reserve item/i }),
      ).not.toBeDisabled();
    });
  });

  it("renders 'Out of Stock' and disables the button when available_to_reserve is 0", () => {
    renderProductCard({ ...FAKE_ROW, available_to_reserve: 0 });

    const button = screen.getByRole("button", { name: /out of stock/i });
    expect(button).toBeDisabled();
    expect(reserveSaleMock).not.toHaveBeenCalled();
  });
});
