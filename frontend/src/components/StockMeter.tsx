type Props = {
  total: number;
  available: number;
};

export function StockMeter({ total, available }: Props) {
  const percent = total === 0 ? 0 : Math.round((available / total) * 100);
  const outOfStock = available <= 0;

  return (
    <div>
      <div className="stock-meter__label">Total Stock Meter</div>
      <div className="stock-meter__bar" aria-hidden="true">
        <div className="stock-meter__fill" style={{ width: `${percent}%` }} />
      </div>
      <div className="stock-meter__row">
        <span>
          {available} / {total} {outOfStock ? "Out of Stock" : "Available"}
        </span>
        <span className="stock-meter__row--muted">{outOfStock ? "0%" : `${percent}%`}</span>
      </div>
    </div>
  );
}
