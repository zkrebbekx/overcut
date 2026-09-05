// Predicted price changes before the next round, with the predictor's
// measured accuracy on this season's real moves.

import { useEffect, useState } from "react";
import { api, type PricesView as Prices } from "../api";
import { Card, Delta, ErrorBox, Spinner, Stat } from "../components/ui";

export function PricesView() {
  const [data, setData] = useState<Prices | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    api.prices().then(setData).catch((e) => setError(String(e.message ?? e)));
  }, []);

  if (error) return <ErrorBox error={error} />;
  if (!data) return <Spinner label="Fitting the price model…" />;

  const risers = data.predictions.filter((p) => p.Change > 0.005);
  const fallers = data.predictions.filter((p) => p.Change < -0.005).reverse();
  const flat = data.predictions.filter((p) => Math.abs(p.Change) <= 0.005);
  const maxAbs = Math.max(0.1, ...data.predictions.map((p) => Math.abs(p.Change)));

  const List = ({ title, items }: { title: string; items: typeof data.predictions }) => (
    <Card title={title}>
      {items.length === 0 && <p className="text-sm text-ink-3">None predicted.</p>}
      <ul className="space-y-1.5">
        {items.map((p) => (
          <li key={p.AssetID} className="flex items-center gap-3 text-sm">
            <span className="w-40 truncate text-ink">{p.Name}</span>
            <span className="num w-12 text-right text-xs text-ink-3">{p.Price.toFixed(1)}</span>
            <span className="relative h-2 flex-1 rounded-full bg-raised" aria-hidden>
              <span className={`absolute top-0 h-2 rounded-full ${p.Change > 0 ? "bg-gain" : "bg-loss"}`} style={{ width: `${(Math.abs(p.Change) / maxAbs) * 100}%` }} />
            </span>
            <span className="w-16 text-right text-xs">
              <Delta value={p.Change} digits={2} suffix="M" />
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <Stat label="Predictor error" value={`$${data.report.MAE.toFixed(2)}M`} sub={`vs $${data.report.NaiveMAE.toFixed(2)}M if you assume no change`} tone="accent" />
        <Stat label="Direction hit rate" value={`${(data.report.Direction * 100).toFixed(0)}%`} sub={`on ${data.report.Moves} real moves this season`} />
        <Stat label="How it works" value={<span className="text-base font-normal text-ink-2">Last-3-round form, fitted on this season's moves</span>} />
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <List title="Likely to rise" items={risers} />
        <List title="Likely to fall" items={fallers} />
      </div>
      {flat.length > 0 && (
        <p className="text-xs text-ink-3">
          Flat: {flat.map((p) => p.Name).join(", ")}
        </p>
      )}
    </div>
  );
}
