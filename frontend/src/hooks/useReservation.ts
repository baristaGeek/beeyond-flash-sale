import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useRef } from "react";
import type { Reservation } from "@/types/api";
import { apiPost, ApiError } from "@/api/client";

type Variables = {
  quantity: number;
};

// useReservation submits POST /api/sales/:saleId/reservations. The
// Idempotency-Key is generated once per hook instance and reused across
// retries — that is what makes network-retry replays hit the same cached
// response on the server.
export function useReservation(saleId: string): UseMutationResult<Reservation, ApiError, Variables> & {
  resetIdempotencyKey: () => void;
} {
  const idemKeyRef = useRef<string>(crypto.randomUUID());

  const mutation = useMutation<Reservation, ApiError, Variables>({
    mutationFn: async ({ quantity }) => {
      return apiPost<Reservation>(
        `/api/sales/${saleId}/reservations`,
        { quantity },
        { idempotencyKey: idemKeyRef.current },
      );
    },
  });

  return Object.assign(mutation, {
    resetIdempotencyKey: () => {
      idemKeyRef.current = crypto.randomUUID();
    },
  });
}
