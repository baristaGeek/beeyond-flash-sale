import type { ApiErrorBody, ApiErrorEnvelope } from "@/types/api";

const SESSION_KEY = "flashsale.sessionId";

function ensureSessionId(): string {
  let id = localStorage.getItem(SESSION_KEY);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(SESSION_KEY, id);
  }
  return id;
}

// ApiError is thrown by the request helpers when the server returns a typed
// error envelope. Components catch it and branch on `body.code`.
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

  // 204 No Content is unused by this API; every response has a JSON body.
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

export function apiGet<T>(path: string): Promise<T> {
  return request<T>("GET", path);
}

export function apiPost<T>(
  path: string,
  body: unknown,
  opts: { idempotencyKey?: string } = {},
): Promise<T> {
  return request<T>("POST", path, {
    body: JSON.stringify(body),
    idempotencyKey: opts.idempotencyKey,
  });
}

export function apiDelete<T>(path: string): Promise<T> {
  return request<T>("DELETE", path);
}

export { ensureSessionId };
