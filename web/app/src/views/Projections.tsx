// Every asset's projected distribution for the round, on a shared axis.

import { useEffect, useMemo, useState } from "react";
import { api, type AssetProjection, type ProjectionView, type RoundView, type SeasonView } from "../api";
import type { PlayerState } from "../store";
import { RangeBar } from "../components/RangeBar";
import { Card, ErrorBox, Segmented, Spinner, TeamEdge } from "../components/ui";
import { AssetDetail } from "../components/AssetDetail";

type Sort = "mean" | "value" | "p90" | "p10" | "price" | "ownership";

export function ProjectionsView({ season, round, state }: { season: SeasonView; round: RoundView; state: PlayerState }) {
  const [data, setData] = useState<ProjectionView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [kind, setKind] = useState<"all" | "driver" | "constructor">("all");
  const [sort, setSort] = useState<Sort>("mean");
  const [open, setOpen] = useState<string | null>(null);

  const condKey = JSON.stringify(state.conditions);
  useEffect(() => {
    setData(null);
    api
      .project(round.round, state.sims, state.conditions)
      .then(setData)
      .catch((e) => setError(String(e.message ?? e)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [round.round, state.sims, condKey]);

  const rows = useMemo(() => {
    if (!data) return [];
    const list = data.assets.filter((a) => kind === "all" || a.kind === kind);
    const key: Record<Sort, (a: AssetProjection) => number> = {
      mean: (a) => a.mean,
      value: (a) => a.mean / a.price,
      p90: (a) => a.p90,
      p10: (a) => a.p10,
      price: (a) => a.price,
      ownership: (a) => a.ownership,
    };
    return [...list].sort((a, b) => key[sort](b) - key[sort](a));
  }, [data, kind, sort]);

  const min = Math.min(0, ...rows.map((r) => r.p10));
  const max = Math.max(1, ...rows.map((r) => r.p90));
  const byId = useMemo(() => new Map(season.assets.map((a) => [a.id, a])), [season]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <Segmented
          value={kind}
          onChange={setKind}
          options={[
            { value: "all", label: "All" },
            { value: "driver", label: "Drivers" },
            { value: "constructor", label: "Constructors" },
          ]}
        />
        <Segmented<Sort>
          value={sort}
          onChange={setSort}
          options={[
            { value: "mean", label: "Expected" },
            { value: "value", label: "Pts / $M" },
            { value: "p10", label: "Floor" },
            { value: "p90", label: "Ceiling" },
            { value: "ownership", label: "Owned" },
          ]}
        />
        <span className="ml-auto text-xs text-ink-3">{data ? `${data.name} · ${data.sims.toLocaleString()} sims${data.has_sprint ? " · sprint weekend" : ""}` : ""}</span>
      </div>

      {error && <ErrorBox error={error} />}
      {!data && !error && <Spinner label={`Running ${state.sims.toLocaleString()} race simulations…`} />}

      {data && (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="text-left text-[11px] uppercase tracking-[0.12em] text-ink-3">
                <tr>
                  <th className="pb-2 pr-3 font-medium">Asset</th>
                  <th className="pb-2 pr-3 text-right font-medium">Price</th>
                  <th className="pb-2 pr-3 text-right font-medium">Expected</th>
                  <th className="pb-2 pr-3 font-medium">
                    <span className="flex justify-between">
                      <span className="num">{min.toFixed(0)}</span>
                      <span>P10 — mean — P90</span>
                      <span className="num">{max.toFixed(0)}</span>
                    </span>
                  </th>
                  <th className="pb-2 pr-3 text-right font-medium">Pts/$M</th>
                  <th className="pb-2 pr-3 text-right font-medium">Owned</th>
                  <th className="pb-2 text-right font-medium">{rows.some((r) => r.has_actual) ? "Actual" : "Last"}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((a) => {
                  const mine = state.team.includes(a.id);
                  return (
                    <tr key={a.id} onClick={() => setOpen(a.id)} className={`cursor-pointer border-t border-line/60 hover:bg-raised/50 ${mine ? "bg-raised/60" : ""}`}>
                      <td className="py-1.5 pr-3">
                        <span className="flex items-center">
                          <TeamEdge team={a.team_name} />
                          <span className={mine ? "font-semibold text-ink" : "text-ink"}>{a.name}</span>
                          {mine && <span className="ml-2 text-[10px] uppercase tracking-wider text-ink-3">yours</span>}
                          {a.kind === "constructor" && <span className="ml-2 text-[10px] uppercase text-ink-3">team</span>}
                        </span>
                      </td>
                      <td className="num py-1.5 pr-3 text-right text-ink-2">{a.price.toFixed(1)}</td>
                      <td className="num py-1.5 pr-3 text-right font-semibold text-ink">{a.mean.toFixed(1)}</td>
                      <td className="min-w-[220px] py-1.5 pr-3">
                        <RangeBar p10={a.p10} p50={a.p50} p90={a.p90} mean={a.mean} min={min} max={max} />
                      </td>
                      <td className="num py-1.5 pr-3 text-right text-ink-2">{(a.mean / a.price).toFixed(2)}</td>
                      <td className="num py-1.5 pr-3 text-right text-ink-2">{a.ownership.toFixed(0)}%</td>
                      <td className="num py-1.5 text-right text-ink-2">
                        {a.has_actual ? (
                          <span title={`projected ${a.mean.toFixed(1)}`} className={Math.abs(a.actual_points - a.mean) <= a.sd ? "text-ink" : "text-warn"}>
                            {a.actual_points.toFixed(0)}
                          </span>
                        ) : (
                          a.last_points.toFixed(0)
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {open && byId.get(open) && <AssetDetail asset={byId.get(open)!} projection={data?.assets.find((a) => a.id === open)} onClose={() => setOpen(null)} />}
    </div>
  );
}
