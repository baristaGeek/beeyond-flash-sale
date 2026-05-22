import { useEffect, useState } from "react";
import { formatMMSS, secondsRemaining, urgencyClass } from "./CountdownTimer.helpers";

type Props = {
  expiresAt: string; // ISO timestamp
  onExpired?: () => void;
};

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
