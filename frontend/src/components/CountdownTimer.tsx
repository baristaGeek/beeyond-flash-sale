import { useEffect, useState } from "react";

type Props = {
  expiresAt: string; // ISO timestamp
  onExpired?: () => void;
};

function secondsRemaining(expiresAt: string): number {
  return Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000));
}

function formatMMSS(seconds: number): string {
  const m = Math.floor(seconds / 60).toString().padStart(2, "0");
  const s = (seconds % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

function urgencyClass(seconds: number): string {
  if (seconds <= 10) return "countdown countdown--danger";
  if (seconds <= 30) return "countdown countdown--warn";
  return "countdown countdown--ok";
}

export function CountdownTimer({ expiresAt, onExpired }: Props) {
  const [remaining, setRemaining] = useState(() => secondsRemaining(expiresAt));

  useEffect(() => {
    const tick = () => {
      const r = secondsRemaining(expiresAt);
      setRemaining(r);
      if (r === 0 && onExpired) onExpired();
    };
    const id = window.setInterval(tick, 250);
    tick();
    return () => window.clearInterval(id);
  }, [expiresAt, onExpired]);

  return <div className={urgencyClass(remaining)}>{formatMMSS(remaining)}</div>;
}
