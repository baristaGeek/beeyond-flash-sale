export function secondsRemaining(expiresAt: string): number {
  return Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000));
}

export function formatMMSS(seconds: number): string {
  const m = Math.floor(seconds / 60).toString().padStart(2, "0");
  const s = (seconds % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

export function urgencyClass(seconds: number): string {
  if (seconds <= 10) return "countdown countdown--danger";
  if (seconds <= 30) return "countdown countdown--warn";
  return "countdown countdown--ok";
}
