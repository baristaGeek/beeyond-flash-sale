type Props = {
  isLive: boolean;
  onRefresh: () => void;
  isRefreshing: boolean;
};

export function Header({ isLive, onRefresh, isRefreshing }: Props) {
  return (
    <header className="header">
      <h1 className="header__title">Atomic Inventory</h1>
      <div className="header__right">
        <span className="status-pill" title={isLive ? "Polling for live data" : "Polling paused"}>
          Status:&nbsp;
          <span className={`status-dot ${isLive ? "" : "status-dot--stale"}`} aria-hidden="true" />
          &nbsp;{isLive ? "Live" : "Paused"}
        </span>
        <button
          type="button"
          className="refresh-button"
          onClick={onRefresh}
          disabled={isRefreshing}
          aria-label="Refresh inventory"
        >
          <span aria-hidden="true">↻</span>
          &nbsp;{isRefreshing ? "Refreshing…" : "Refresh"}
        </button>
      </div>
    </header>
  );
}
