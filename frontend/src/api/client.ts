import type {
  ApiErrorBody,
  ApiErrorEnvelope,
  InventoryListResponse,
  InventoryRow,
  ReleaseResponse,
  Reservation,
} from "@/types/api";

const SESSION_KEY = "flashsale.sessionId";

function ensureSessionId(): string {
  let id = localStorage.getItem(SESSION_KEY);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(SESSION_KEY, id);
  }
  return id;
}

export class ApiError extends Error {
  body: ApiErrorBody;
  status: number;

  constructor(status: number, body: ApiErrorBody) {
    super(`${body.code}: ${body.message}`);
    this.status = status;
    this.body = body;
  }
}

type RequestInitWithIdempotency = RequestInit & {
  idempotencyKey?: string;
};

async function request<T>(
  method: string,
  path: string,
  init: RequestInitWithIdempotency = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("X-Session-Id", ensureSessionId());
  if (init.body) {
    headers.set("Content-Type", "application/json");
  }
  if (init.idempotencyKey) {
    headers.set("Idempotency-Key", init.idempotencyKey);
  }

  const res = await fetch(path, { ...init, method, headers });
  const text = await res.text();
  const json = text ? (JSON.parse(text) as unknown) : null;

  if (!res.ok) {
    if (json && typeof json === "object" && "error" in json) {
      const env = json as ApiErrorEnvelope;
      throw new ApiError(res.status, env.error);
    }
    throw new ApiError(res.status, {
      code: "INTERNAL",
      message: `Unexpected response (HTTP ${res.status}).`,
    });
  }

  return json as T;
}

export function listSales(): Promise<InventoryListResponse> {
  return request<InventoryListResponse>("GET", "/api/sales");
}

export function getInventory(saleId: string): Promise<InventoryRow> {
  return request<InventoryRow>("GET", `/api/sales/${saleId}/inventory`);
}

export function reserveSale(
  saleId: string,
  quantity: number,
  idempotencyKey: string,
): Promise<Reservation> {
  return request<Reservation>(
    "POST",
    `/api/sales/${saleId}/reservations`,
    {
      body: JSON.stringify({ quantity }),
      idempotencyKey,
    },
  );
}

export function getReservation(reservationId: string): Promise<Reservation> {
  return request<Reservation>("GET", `/api/reservations/${reservationId}`);
}

export function releaseReservation(reservationId: string): Promise<ReleaseResponse> {
  return request<ReleaseResponse>("DELETE", `/api/reservations/${reservationId}`);
}

export { ensureSessionId };
