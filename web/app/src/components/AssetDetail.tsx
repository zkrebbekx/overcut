// A side panel with one asset's season history: points and price per
// gameday, plus the current projection.

import { useEffect } from "react";
import { X } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { AssetProjection, AssetView } from "../api";
import { chart } from "./chartTheme";

export function AssetDetail({ asset, projection, onClose }: { asset: AssetView; projection?: AssetProjection; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const data = asset.history.map((h) => ({ gameday: `R${h.gameday}`, points: h.points, price: h.price, quali: h.quali_pts, sprint: h.sprint_pts, race: h.race_pts }));

  return (
    <div role="dialog" aria-modal aria-label={`${asset.name} detail`} className="fixed inset-0 z-20 flex justify-end bg-bg/60" onClick={onClose}>
      <aside className="fade-in h-full w-full max-w-md overflow-y-auto border-l border-line bg-panel p-5" onClick={(e) => e.stopPropagation()}>
        <header className="mb-4 flex items-start justify-between">
          <div>
            <h2 className="display text-xl font-semibold text-ink">{asset.name}</h2>
            <p className="text-xs text-ink-3">
              {asset.team_name} · {asset.kind} · <span className="num">${asset.price.toFixed(1)}M</span> · {asset.ownership.toFixed(0)}% owned
            </p>
          </div>
          <button onClick={onClose} aria-label="Close" className="rounded p-1 text-ink-3 hover:text-ink">
            <X size={18} />
          </button>
        </header>

        {projection && (
          <div className="mb-5 grid grid-cols-4 gap-2 text-center">
            {[
              ["P10", projection.p10],
              ["Median", projection.p50],
              ["Mean", projection.mean],
              ["P90", projection.p90],
            ].map(([k, v]) => (
              <div key={k as string} className="chip py-2">
                <div className="text-[10px] uppercase tracking-wider text-ink-3">{k}</div>
                <div className="num text-lg font-semibold text-ink">{(v as number).toFixed(0)}</div>
              </div>
            ))}
          </div>
        )}

        <h3 className="mb-2 text-xs uppercase tracking-[0.12em] text-ink-3">Points per round</h3>
        <div className="h-44">
          <ResponsiveContainer>
            <BarChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
              <CartesianGrid stroke={chart.grid} vertical={false} />
              <XAxis dataKey="gameday" tick={chart.tick} axisLine={false} tickLine={false} />
              <YAxis tick={chart.tick} axisLine={false} tickLine={false} />
              <Tooltip contentStyle={chart.tooltip} cursor={{ fill: chart.cursor }} />
              <Bar dataKey="points" name="Points" fill={chart.s1} radius={[4, 4, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        <h3 className="mb-2 mt-5 text-xs uppercase tracking-[0.12em] text-ink-3">Price</h3>
        <div className="h-36">
          <ResponsiveContainer>
            <LineChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
              <CartesianGrid stroke={chart.grid} vertical={false} />
              <XAxis dataKey="gameday" tick={chart.tick} axisLine={false} tickLine={false} />
              <YAxis tick={chart.tick} axisLine={false} tickLine={false} domain={["auto", "auto"]} />
              <Tooltip contentStyle={chart.tooltip} />
              <Line type="monotone" dataKey="price" name="Price ($M)" stroke={chart.s1} strokeWidth={2} dot={{ r: 3 }} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <table className="mt-5 w-full text-xs">
          <thead className="text-left text-[10px] uppercase tracking-wider text-ink-3">
            <tr>
              <th className="pb-1 font-medium">Round</th>
              <th className="pb-1 text-right font-medium">Q</th>
              <th className="pb-1 text-right font-medium">Sprint</th>
              <th className="pb-1 text-right font-medium">Race</th>
              <th className="pb-1 text-right font-medium">Total</th>
              <th className="pb-1 text-right font-medium">Price</th>
            </tr>
          </thead>
          <tbody className="num text-ink-2">
            {asset.history.map((h) => (
              <tr key={h.gameday} className="border-t border-line/60">
                <td className="py-1">R{h.gameday}</td>
                <td className="py-1 text-right">{h.quali_pts}</td>
                <td className="py-1 text-right">{h.sprint_pts || "—"}</td>
                <td className="py-1 text-right">{h.race_pts}</td>
                <td className="py-1 text-right font-semibold text-ink">{h.points}</td>
                <td className="py-1 text-right">{h.price.toFixed(1)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </aside>
    </div>
  );
}
