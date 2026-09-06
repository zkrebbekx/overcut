// Model accuracy, measured walk-forward on this season.

import { useEffect, useState } from "react";
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api, type BacktestReport } from "../api";
import { Card, ErrorBox, Spinner, Stat } from "../components/ui";
import { chart } from "../components/chartTheme";

export function TrustView() {
  const [data, setData] = useState<BacktestReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    api.backtest(3000).then(setData).catch((e) => setError(String(e.message ?? e)));
  }, []);

  if (error) return <ErrorBox error={error} />;
  if (!data) return <Spinner label="Replaying the season: fitting and simulating each past round…" />;

  const gap = data.HindsightTeamPts - data.NaiveTeamPts;
  const captured = gap > 0 ? ((data.ModelTeamPts - data.NaiveTeamPts) / gap) * 100 : 0;
  const rows = data.Rounds.map((r) => ({ name: `R${r.Round}`, Model: Math.round(r.ModelTeamPts), "Last-round picks": Math.round(r.NaiveTeamPts), Hindsight: Math.round(r.HindsightTeamPts) }));

  return (
    <div className="space-y-4">
      <p className="max-w-3xl text-sm leading-relaxed text-ink-2">
        For each finished round from R4 onward, the model was fitted only on earlier rounds, projected the round, and was scored against the official points. Two naive
        strategies are scored the same way so the edge is visible.
      </p>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="Driver error (MAE)" value={`±${data.DriverMAE.toFixed(1)}`} sub={`last-round baseline ±${data.BaselinePrev.toFixed(1)} · season-mean ±${data.BaselineSeason.toFixed(1)}`} tone="accent" />
        <Stat label="Driver rank correlation" value={data.MeanSpearman.toFixed(2)} sub="Spearman ρ, 1.0 = perfect order" />
        <Stat label="Team points per round" value={data.ModelTeamPts.toFixed(0)} sub={`naive ${data.NaiveTeamPts.toFixed(0)} · hindsight limit ${data.HindsightTeamPts.toFixed(0)}`} tone="gain" />
        <Stat label="Of the possible gain" value={`${captured.toFixed(0)}%`} sub="captured between naive and hindsight" />
      </div>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="Error with the grid known" value={`±${data.GridDriverMAE.toFixed(1)}`} sub="the Sunday-morning forecast, after qualifying, sprint, and penalties" tone="accent" />
        <Stat label="Rank correlation with the grid known" value={data.GridMeanSpearman.toFixed(2)} sub={`from ${data.MeanSpearman.toFixed(2)} before qualifying`} />
        <Stat label="Team points with the grid known" value={data.GridTeamPts.toFixed(0)} sub={`from ${data.ModelTeamPts.toFixed(0)} before qualifying`} tone="gain" />
        <Stat label="P10–P90 range coverage" value={`${(data.Coverage * 100).toFixed(0)}%`} sub={`${(data.GridCoverage * 100).toFixed(0)}% with the grid known · a calibrated range covers 80%`} />
      </div>

      <Card title="Pre-race optimal team, scored with real points">
        <div className="h-64">
          <ResponsiveContainer>
            <BarChart data={rows} margin={{ top: 8, right: 8, left: -16, bottom: 0 }} barGap={2}>
              <CartesianGrid stroke={chart.grid} vertical={false} />
              <XAxis dataKey="name" tick={chart.tick} axisLine={false} tickLine={false} />
              <YAxis tick={chart.tick} axisLine={false} tickLine={false} />
              <Tooltip contentStyle={chart.tooltip} cursor={{ fill: chart.cursor }} />
              <Legend wrapperStyle={{ fontSize: 12 }} formatter={(v) => <span style={{ color: "#a7afbc" }}>{v}</span>} />
              <Bar dataKey="Model" fill={chart.s1} radius={[4, 4, 0, 0]} isAnimationActive={false} />
              <Bar dataKey="Last-round picks" fill={chart.s2} radius={[4, 4, 0, 0]} isAnimationActive={false} />
              <Bar dataKey="Hindsight" fill={chart.s3} radius={[4, 4, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </Card>

      <Card title="Per round">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="text-left text-[11px] uppercase tracking-[0.12em] text-ink-3">
              <tr>
                <th className="pb-2 pr-3 font-medium">Round</th>
                <th className="pb-2 pr-3 text-right font-medium">Driver MAE</th>
                <th className="pb-2 pr-3 text-right font-medium">Grid known</th>
                <th className="pb-2 pr-3 text-right font-medium">Constructor MAE</th>
                <th className="pb-2 pr-3 text-right font-medium">Rank ρ</th>
                <th className="pb-2 pr-3 text-right font-medium">Model team</th>
                <th className="pb-2 pr-3 text-right font-medium">Grid known</th>
                <th className="pb-2 pr-3 text-right font-medium">Naive team</th>
                <th className="pb-2 text-right font-medium">Hindsight</th>
              </tr>
            </thead>
            <tbody className="num text-ink-2">
              {data.Rounds.map((r) => (
                <tr key={r.Round} className="border-t border-line/60">
                  <td className="py-1.5 pr-3 font-sans text-ink">
                    R{r.Round} {r.Name}
                  </td>
                  <td className="py-1.5 pr-3 text-right">{r.DriverMAE.toFixed(1)}</td>
                  <td className="py-1.5 pr-3 text-right">{r.GridDriverMAE.toFixed(1)}</td>
                  <td className="py-1.5 pr-3 text-right">{r.ConsMAE.toFixed(1)}</td>
                  <td className="py-1.5 pr-3 text-right">{r.SpearmanRho.toFixed(2)}</td>
                  <td className="py-1.5 pr-3 text-right font-semibold text-ink">{r.ModelTeamPts.toFixed(0)}</td>
                  <td className="py-1.5 pr-3 text-right">{r.GridTeamPts.toFixed(0)}</td>
                  <td className="py-1.5 pr-3 text-right">{r.NaiveTeamPts.toFixed(0)}</td>
                  <td className="py-1.5 text-right">{r.HindsightTeamPts.toFixed(0)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}
