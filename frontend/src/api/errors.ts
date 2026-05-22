import type { ApiErrorBody, ApiErrorCode } from "@/types/api";
import { ApiError } from "./client";

export function isApiError(e: unknown): e is ApiError {
  return e instanceof ApiError;
}

// Default human-readable labels per error code. Components are free to
// override these with context-specific phrasing.
export const defaultErrorLabel: Record<ApiErrorCode, string> = {
  INVALID_QUANTITY: "Please enter a valid quantity.",
  SALE_NOT_FOUND: "This sale is no longer available.",
  RESERVATION_NOT_FOUND: "This reservation is no longer active.",
  RESERVATION_TERMINAL: "This reservation was already released or expired.",
  INSUFFICIENT_STOCK: "Not enough stock to fill this reservation.",
  IDEMPOTENCY_KEY_MISMATCH:
    "We already processed a different request with this attempt key. Please start a fresh attempt.",
  LOCK_TIMEOUT: "The system is busy — please try again in a moment.",
  VALIDATION: "Your request couldn't be processed; please check your input.",
  INTERNAL: "Something went wrong on our side. Please try again.",
};

export function describeError(err: unknown): { code: ApiErrorCode; body: ApiErrorBody } {
  if (isApiError(err)) {
    return { code: err.body.code, body: err.body };
  }
  return {
    code: "INTERNAL",
    body: { code: "INTERNAL", message: defaultErrorLabel.INTERNAL },
  };
}
