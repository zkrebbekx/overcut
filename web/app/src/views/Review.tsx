// Post-round review: the grid-known projection against the official
// points, ranked by surprise, plus how the player's team did.

import { useEffect, useMemo, useState } from "react";
import { api, type ReviewAsset, type ReviewView as Review, type SeasonView } from "../api";
import type { PlayerState } from "../store";
import { Card, Delta, ErrorBox, Segmented, Spinner, Stat, TeamEdge } from "../components/ui";

type Sort = "surprise" | "actual" | "projected" | "z";

export function ReviewView({ season, state }: { season: SeasonView; state: PlayerState }) {
  const done = season.rounds.filter((r) => r.has_results);
  const [round, setRound] = useState(done[done.length - 1]?.round ?? 1);
  const [data, setData] = useState<Review | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sort, setSort] = useState<Sort>("surprise");
  const [kind, setKind] = useState<"all" | "driver" | "constructor">("all");

  const teamKey = state.team.join(",");
  useEffect(() => {
    setData(null);
    setError(null);
    api
      .review({ round, sims: 20000, team: state.team })
      .then(setData)
      .catch((e) => setError(String(e.message ?? e)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [round, teamKey]);

  const rows = useMemo(() => {
    if (!data) return [];
    const list = data.assets.filter((a) => kind === "all" || a.kind === kind);
    const key: Record<Sort, (a: ReviewAsset) => number> = {
      surprise: (a) => Math.abs(a.delta),
      actual: (a) => a.actual,
      projected: (a) => a.projected,
      z: (a) => Math.abs(a.z),
    };
    return [...list].sort((a, b) => key[sort](b) - key[sort](a));
  }, [data, sort, kind]);

  const over = data?.assets.filter((a) => a.delta > 0).slice(0, 3) ?? [];
  const under = data?.assets.filter((a) => a.delta < 0).slice(0, 3) ?? [];
  const heldCount = data?.assets.filter((a) => a.held).length ?? 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <select aria-label="Round" value={round} onChange={(e) => setRound(Number(e.target.value))} className="chip px-2 py-1 text-sm text-ink">
          {done.map((r) => (
            <option key={r.round} value={r.round}>
              R{r.round} · {r.name}
              {r.has_sprint ? " · Sprint" : ""}
            </option>
          ))}
        </select>
        <span className="text-xs text-ink-3">What the model expected on Sunday morning (grid known) against what happened.</span>
        {data?.provisional && (
          <span className="chip px-2 py-1 text-xs text-warn" title="The game publishes provisional points on race day and finalises them within about a day. This review updates automatically.">
            Provisional points
          </span>
        )}
      </div>

      {error && <ErrorBox error={error} />}
      {!data && !error && <Spinner label="Re-running the round with the grid known…" />}

      {data && (
        <>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Stat label="Driver error this round" value={`±${data.driver_mae.toFixed(1)}`} sub="mean absolute error, grid known" tone="mark" />
            <Stat label="Inside the P10–P90 range" value={`${(data.coverage * 100).toFixed(0)}%`} sub="of drivers · a calibrated range covers 80%" />
            {heldCount === 7 ? (
              <Stat label="Your team" value={data.team_actual.toFixed(0)} sub={`projected ${data.team_projected.toFixed(0)} · ${data.captain_id ? "Boost on the best projected driver" : ""}`} tone={data.team_actual >= data.team_projected ? "gain" : "loss"} />
            ) : (
              <Stat label="Your team" value="—" sub="Enter your seven assets on Decide to review them here" />
            )}
            <Stat label="Hindsight optimum" value={data.hindsight_points.toFixed(0)} sub="best possible team at the round's prices" />
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card title="Beat the projection">
              <ul className="space-y-1.5 text-sm">
                {over.map((a) => (
                  <li key={a.id} className="flex items-center gap-3">
                    <span className="flex min-w-0 flex-1 items-center truncate text-ink">
                      <TeamEdge team={a.team_name} />
                      {a.name}
                    </span>
                    <span className="num text-xs text-ink-3">
                      {a.projected.toFixed(0)} → {a.actual.toFixed(0)}
                    </span>
                    <Delta value={a.delta} digits={0} />
                  </li>
                ))}
              </ul>
            </Card>
            <Card title="Missed the projection">
              <ul className="space-y-1.5 text-sm">
                {under.map((a) => (
                  <li key={a.id} className="flex items-center gap-3">
                    <span className="flex min-w-0 flex-1 items-center truncate text-ink">
                      <TeamEdge team={a.team_name} />
                      {a.name}
                    </span>
                    <span className="num text-xs text-ink-3">
                      {a.projected.toFixed(0)} → {a.actual.toFixed(0)}
                    </span>
                    <Delta value={a.delta} digits={0} />
                  </li>
                ))}
              </ul>
            </Card>
          </div>

          <Card
            title="Every asset"
            right={
              <div className="flex flex-wrap gap-2">
                <Segmented
                  value={kind}
                  onChange={setKind}
                  options={[
                    { value: "all", label: "All" },
                    { value: "driver", label: "Drivers" },
                    { value: "constructor", label: "Teams" },
                  ]}
                />
                <Segmented<Sort>
                  value={sort}
                  onChange={setSort}
                  options={[
                    { value: "surprise", label: "Surprise" },
                    { value: "z", label: "In SDs" },
                    { value: "actual", label: "Actual" },
                    { value: "projected", label: "Projected" },
                  ]}
                />
              </div>
            }
          >
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-left text-[11px] uppercase tracking-[0.12em] text-ink-3">
                  <tr>
                    <th className="pb-2 pr-3 font-medium">Asset</th>
                    <th className="pb-2 pr-3 text-right font-medium">Price</th>
                    <th className="pb-2 pr-3 text-right font-medium">Projected</th>
                    <th className="pb-2 pr-3 text-right font-medium">P10–P90</th>
                    <th className="pb-2 pr-3 text-right font-medium">Actual</th>
                    <th className="pb-2 pr-3 text-right font-medium">Delta</th>
                    <th className="pb-2 text-right font-medium">SDs</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((a) => (
                    <tr key={a.id} className={`border-t border-line/60 ${a.held ? "bg-raised/60" : ""}`}>
                      <td className="py-1.5 pr-3">
                        <span className="flex items-center">
                          <TeamEdge team={a.team_name} />
                          <span className={a.held ? "font-semibold text-ink" : "text-ink"}>{a.name}</span>
                          {a.held && <span className="ml-2 text-[10px] uppercase tracking-wider text-ink-3">yours</span>}
                          {a.kind === "constructor" && <span className="ml-2 text-[10px] uppercase text-ink-3">team</span>}
                        </span>
                      </td>
                      <td className="num py-1.5 pr-3 text-right text-ink-2">{a.price.toFixed(1)}</td>
                      <td className="num py-1.5 pr-3 text-right text-ink-2">{a.projected.toFixed(1)}</td>
                      <td className="num py-1.5 pr-3 text-right text-ink-3">
                        {a.p10.toFixed(0)}–{a.p90.toFixed(0)}
                      </td>
                      <td className={`num py-1.5 pr-3 text-right font-semibold ${a.in_range ? "text-ink" : "text-warn"}`}>{a.actual.toFixed(0)}</td>
                      <td className="py-1.5 pr-3 text-right">
                        <Delta value={a.delta} digits={1} />
                      </td>
                      <td className="num py-1.5 text-right text-ink-2">{a.z >= 0 ? "+" : ""}{a.z.toFixed(1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className="mt-2 text-xs text-ink-3">Amber actuals fell outside the projected P10–P90 range. Highlighted rows are on your team.</p>
          </Card>
        </>
      )}
    </div>
  );
}
