import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { CountdownTimer } from "./CountdownTimer";
import { formatMMSS, secondsRemaining, urgencyClass } from "./CountdownTimer.helpers";

const NOW = new Date("2026-05-22T14:30:00Z");

describe("secondsRemaining", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("floors remaining seconds for a future expiry", () => {
    const expiresAt = new Date(NOW.getTime() + 59_750).toISOString();
    expect(secondsRemaining(expiresAt)).toBe(59);
  });

  it("returns 0 for an expiry in the past (clamped, not negative)", () => {
    const expiresAt = new Date(NOW.getTime() - 10_000).toISOString();
    expect(secondsRemaining(expiresAt)).toBe(0);
  });

  it("returns 0 when expiry equals now", () => {
    expect(secondsRemaining(NOW.toISOString())).toBe(0);
  });

  it("returns the exact whole second when divisible by 1000", () => {
    const expiresAt = new Date(NOW.getTime() + 60_000).toISOString();
    expect(secondsRemaining(expiresAt)).toBe(60);
  });
});

describe("formatMMSS", () => {
  it("pads zero", () => {
    expect(formatMMSS(0)).toBe("00:00");
  });

  it("pads single-digit seconds", () => {
    expect(formatMMSS(7)).toBe("00:07");
  });

  it("formats sub-minute values", () => {
    expect(formatMMSS(59)).toBe("00:59");
  });

  it("rolls over to minutes at 60s", () => {
    expect(formatMMSS(60)).toBe("01:00");
  });

  it("formats multi-minute values", () => {
    expect(formatMMSS(605)).toBe("10:05");
  });
});

describe("urgencyClass", () => {
  it("is 'ok' above the warn threshold", () => {
    expect(urgencyClass(31)).toBe("countdown countdown--ok");
  });

  it("becomes 'warn' at exactly 30s (inclusive boundary)", () => {
    expect(urgencyClass(30)).toBe("countdown countdown--warn");
  });

  it("stays 'warn' just above the danger threshold", () => {
    expect(urgencyClass(11)).toBe("countdown countdown--warn");
  });

  it("becomes 'danger' at exactly 10s (inclusive boundary)", () => {
    expect(urgencyClass(10)).toBe("countdown countdown--danger");
  });

  it("is 'danger' at zero", () => {
    expect(urgencyClass(0)).toBe("countdown countdown--danger");
  });
});

describe("<CountdownTimer />", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  function expiresIn(ms: number): string {
    return new Date(NOW.getTime() + ms).toISOString();
  }

  it("renders the initial remaining time and 'ok' class for a 60s expiry", () => {
    const { container } = render(<CountdownTimer expiresAt={expiresIn(60_000)} />);
    const node = container.firstElementChild as HTMLElement;
    expect(node).toHaveTextContent("01:00");
    expect(node).toHaveClass("countdown", "countdown--ok");
  });

  it("ticks down and transitions through warn and danger classes", () => {
    const { container } = render(<CountdownTimer expiresAt={expiresIn(60_000)} />);
    const node = container.firstElementChild as HTMLElement;

    // Advance to t=31s remaining → still 'ok' boundary check: 31 > 30 ⇒ ok.
    act(() => {
      vi.advanceTimersByTime(29_000);
    });
    expect(node).toHaveTextContent("00:31");
    expect(node).toHaveClass("countdown--ok");

    // Advance 1s → 30s remaining, crosses into 'warn'.
    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(node).toHaveTextContent("00:30");
    expect(node).toHaveClass("countdown--warn");

    // Advance to t=10s remaining → crosses into 'danger'.
    act(() => {
      vi.advanceTimersByTime(20_000);
    });
    expect(node).toHaveTextContent("00:10");
    expect(node).toHaveClass("countdown--danger");
  });

  it("invokes onExpired once the timer has crossed expiry", () => {
    const onExpired = vi.fn();
    render(<CountdownTimer expiresAt={expiresIn(60_000)} onExpired={onExpired} />);

    // Long before expiry — not yet called.
    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(onExpired).not.toHaveBeenCalled();

    // Past expiry — fires. Note the production component fires on every
    // tick once `remaining === 0`, so we assert "at least once" rather than
    // pinning a specific call count.
    act(() => {
      vi.advanceTimersByTime(31_000);
    });
    expect(onExpired).toHaveBeenCalled();
  });

  it("displays 00:00 at and beyond expiry", () => {
    const { container } = render(<CountdownTimer expiresAt={expiresIn(1_000)} />);
    const node = container.firstElementChild as HTMLElement;

    act(() => {
      vi.advanceTimersByTime(2_000);
    });
    expect(node).toHaveTextContent("00:00");
    expect(node).toHaveClass("countdown--danger");
  });

  it("stops invoking onExpired after unmount", () => {
    const onExpired = vi.fn();
    const { unmount } = render(
      <CountdownTimer expiresAt={expiresIn(500)} onExpired={onExpired} />,
    );

    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    const callsBeforeUnmount = onExpired.mock.calls.length;
    expect(callsBeforeUnmount).toBeGreaterThan(0);

    unmount();

    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    expect(onExpired).toHaveBeenCalledTimes(callsBeforeUnmount);
  });
});
