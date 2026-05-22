import { useCallback, useEffect, useState } from "react";
import { getReservation, ApiError } from "@/api/client";
import type { Reservation } from "@/types/api";

const STORAGE_KEY = "flashsale.reservations";

function readIds(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((s) => typeof s === "string") : [];
  } catch {
    return [];
  }
}

function writeIds(ids: string[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(ids));
}

/**
 * useReservationsTracker keeps a localStorage-backed list of the user's
 * reservation IDs, polls each one against GET /api/reservations/{id}, and
 * exposes the resulting list of active Reservation objects.
 *
 * Polling cadence: 500ms (twice per second) — well inside the 1-second
 * freshness budget and tight enough that the countdown timer never lags the
 * server by more than half a tick.
 *
 * Reservations that the server reports as terminal (or refuses to return) are
 * automatically dropped from local storage so the panel doesn't carry stale
 * cards across reloads.
 */
export function useReservationsTracker() {
  const [ids, setIds] = useState<string[]>(readIds);
  const [reservations, setReservations] = useState<Reservation[]>([]);

  const trackReservation = useCallback((reservation: Reservation) => {
    setIds((current) => {
      if (current.includes(reservation.id)) return current;
      const next = [...current, reservation.id];
      writeIds(next);
      return next;
    });
    setReservations((current) =>
      current.some((r) => r.id === reservation.id)
        ? current
        : [...current, reservation],
    );
  }, []);

  const dropReservation = useCallback((id: string) => {
    setIds((current) => {
      const next = current.filter((rid) => rid !== id);
      writeIds(next);
      return next;
    });
    setReservations((current) => current.filter((r) => r.id !== id));
  }, []);

  // Poll each tracked reservation. We don't go through TanStack Query here
  // because the list of ids is dynamic per render and a custom poll is
  // simpler to reason about than `useQueries` for this small N.
  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      if (ids.length === 0) {
        if (!cancelled) setReservations([]);
        return;
      }
      const results = await Promise.allSettled(ids.map((id) => getReservation(id)));
      if (cancelled) return;
      const next: Reservation[] = [];
      const drop: string[] = [];
      results.forEach((res, idx) => {
        const id = ids[idx];
        if (!id) return;
        if (res.status === "fulfilled") {
          if (res.value.status === "active") {
            next.push(res.value);
          } else {
            // Terminal — drop from storage.
            drop.push(id);
          }
        } else {
          if (res.reason instanceof ApiError && res.reason.body.code === "RESERVATION_NOT_FOUND") {
            drop.push(id);
          }
          // Other errors (network, lock_timeout) — keep the id around; we'll retry on the next tick.
        }
      });
      if (drop.length > 0) {
        setIds((current) => {
          const filtered = current.filter((id) => !drop.includes(id));
          writeIds(filtered);
          return filtered;
        });
      }
      setReservations(next);
    };
    tick();
    const interval = window.setInterval(tick, 500);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [ids]);

  return {
    reservations,
    trackReservation,
    dropReservation,
  };
}
