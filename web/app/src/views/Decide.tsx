// The home screen: the recommended move for the selected round.

import { useEffect, useMemo, useState } from "react";
import { ArrowRight } from "lucide-react";
import { api, type OptimizeView, type RoundView, type SeasonView, type TeamView } from "../api";
import type { PlayerState } from "../store";
import { TeamBuilder } from "../components/TeamBuilder";
import { ConditionsPanel } from "../components/Conditions";
import { RangeBar } from "../components/RangeBar";
import { Card, Delta, ErrorBox, Spinner, TeamEdge, money } from "../components/ui";

export function DecideView({ season, round, state, update }: { season: SeasonView; round: RoundView; state: PlayerState; update: (p: Partial<PlayerState>) => void }) {
  const [result, setResult] = useState<OptimizeView | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pick, setPick] = useState(0);

  const byId = useMemo(() => new Map(season.assets.map((a) => [a.id, a])), [season]);
  const teamValue = state.team.reduce((s, id) => s + (byId.get(id)?.price ?? 0), 0);
  const complete = state.team.filter((id) => byId.get(id)?.kind === "driver").length === 5 && state.team.filter((id) => byId.get(id)?.kind === "constructor").length === 2;
  const budget = complete ? teamValue + state.cash : 100;

  const requestKey = JSON.stringify({ r: round.round, t: state.team, f: state.freeTransfers, b: budget, c: state.chip, k: state.risk, s: state.sims, w: state.conditions });

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .optimize({
        round: round.round,
        sims: state.sims,
        team: complete ? state.team : [],
        free_transfers: state.freeTransfers,
        budget,
        chip: state.chip,
        risk: state.risk,
        top: 6,
        conditions: state.conditions,
      })
      .then((r) => {
        if (!cancelled) {
          setResult(r);
          setPick(0);
        }
      })
      .catch((e) => !cancelled && setError(String(e.message ?? e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestKey]);

  const best = result?.teams[pick];
  const projById = useMemo(() => new Map(result?.projection.assets.map((a) => [a.id, a]) ?? []), [result]);
  const teamRange = useMemo(() => (best ? rangeOf(best, projById) : null), [best, projById]);
  const keepDelta = best && complete ? best.score - result!.current_score : null;
  const moveIn = best?.in ?? [];
  const moveOut = best?.out ?? [];

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
      <div className="space-y-4">
        <Card
          title={complete ? "Recommended move" : "Best team for the budget"}
          right={<span className="text-xs text-ink-3">{loading ? <Spinner label={`Running ${state.sims.toLocaleString()} simulations…`} /> : result ? `${result.projection.sims.toLocaleString()} sims · ${round.name}` : ""}</span>}
        >
          {error && <ErrorBox error={error} />}
          {best && result && (
            <div className="space-y-4">
              <div className="flex flex-wrap items-end gap-6">
                <div>
                  <div className="text-xs uppercase tracking-[0.12em] text-ink-3">Projected</div>
                  <div className="display num text-3xl font-bold text-ink md:text-4xl">{best.score.toFixed(1)}</div>
                </div>
                {keepDelta !== null && (
                  <div>
                    <div className="text-xs uppercase tracking-[0.12em] text-ink-3">vs keeping your team</div>
                    <div className="display text-2xl font-semibold">
                      <Delta value={keepDelta} />
                    </div>
                  </div>
                )}
                <div>
                  <div className="text-xs uppercase tracking-[0.12em] text-ink-3">Cost</div>
                  <div className="display num text-2xl font-semibold text-ink">{money(best.cost)}</div>
                  <div className="text-xs text-ink-3">of {money(result.budget)}</div>
                </div>
                {complete && (
                  <div>
                    <div className="text-xs uppercase tracking-[0.12em] text-ink-3">Transfers</div>
                    <div className="display num text-2xl font-semibold text-ink">
                      {best.transfers}
                      {best.penalty !== 0 && <span className="ml-2 text-base text-loss">{best.penalty.toFixed(0)} pts</span>}
                    </div>
                  </div>
                )}
              </div>

              {teamRange && (
                <div>
                  <div className="mb-1 flex justify-between text-xs text-ink-3">
                    <span className="num">Floor (P10) {teamRange.p10.toFixed(0)}</span>
                    <span className="num">Ceiling (P90) {teamRange.p90.toFixed(0)}</span>
                  </div>
                  <RangeBar p10={teamRange.p10} p50={teamRange.p50} p90={teamRange.p90} mean={best.score} min={Math.min(teamRange.p10, 0)} max={teamRange.p90 * 1.05} />
                  <p className="mt-1 text-xs text-ink-3">Range sums each asset's percentile; real outcomes correlate, so treat it as a guide, not a bound.</p>
                </div>
              )}

              {complete && (moveIn.length > 0 || moveOut.length > 0) && (
                <ul className="space-y-2">
                  {moveOut.map((outId, i) => {
                    const inId = moveIn[i];
                    const o = byId.get(outId);
                    const n = inId ? byId.get(inId) : undefined;
                    const op = projById.get(outId);
                    const np = inId ? projById.get(inId) : undefined;
                    return (
                      <li key={outId} className="chip flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-sm">
                        <span className="w-10 text-[10px] uppercase tracking-wider text-loss">Out</span>
                        <span className="flex items-center text-ink-2">
                          <TeamEdge team={o?.team_name ?? ""} />
                          {o?.name}
                          <span className="num ml-2 text-xs text-ink-3">{op?.mean.toFixed(1)}</span>
                        </span>
                        <ArrowRight size={14} className="text-ink-3" />
                        <span className="w-10 text-[10px] uppercase tracking-wider text-gain">In</span>
                        <span className="flex items-center text-ink">
                          <TeamEdge team={n?.team_name ?? ""} />
                          {n?.name}
                          <span className="num ml-2 text-xs text-ink-3">{np?.mean.toFixed(1)}</span>
                        </span>
                        {op && np && (
                          <span className="ml-auto text-xs">
                            <Delta value={np.mean - op.mean} />
                          </span>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}
              {complete && moveIn.length === 0 && <p className="text-sm text-gain">Keep your team. No transfer improves the projection.</p>}

              <TeamLine team={best} byId={byId} projById={projById} chip={result.chip} />
            </div>
          )}
          {!best && !loading && !error && <p className="text-sm text-ink-3">No legal team fits the budget.</p>}
        </Card>

        {result && result.teams.length > 1 && (
          <Card title="Alternatives">
            <ol className="space-y-1">
              {result.teams.map((t, i) => (
                <li key={i}>
                  <button
                    onClick={() => setPick(i)}
                    aria-pressed={pick === i}
                    className={`flex w-full items-center gap-3 rounded-[8px] px-3 py-2 text-left text-sm transition ${pick === i ? "bg-raised text-ink" : "text-ink-2 hover:bg-raised/60"}`}
                  >
                    <span className="num w-6 text-ink-3">#{i + 1}</span>
                    <span className="num w-14 font-semibold">{t.score.toFixed(1)}</span>
                    <span className="min-w-0 flex-1 text-xs leading-snug sm:truncate sm:text-sm">
                      {[...t.drivers, ...t.constructors]
                        .map((a) => (a.kind === "driver" ? byId.get(a.id)?.tla ?? a.name : a.name) + (a.id === t.captain_id ? "★" : a.id === t.boost_id ? "+" : ""))
                        .join(" · ")}
                    </span>
                    <span className="num hidden text-xs text-ink-3 sm:inline">{money(t.cost)}</span>
                    {complete && (
                      <span className="num w-16 text-right text-xs text-ink-3">
                        {t.transfers} tr{t.penalty !== 0 ? ` ${t.penalty.toFixed(0)}` : ""}
                      </span>
                    )}
                  </button>
                </li>
              ))}
            </ol>
          </Card>
        )}
      </div>

      <div className="space-y-4">
        <Card title="Your team" right={<span className="num text-xs text-ink-3">{complete ? money(budget) + " budget" : ""}</span>}>
          <TeamBuilder season={season} state={state} update={update} />
        </Card>
        <Card title="What the model knows">
          <ConditionsPanel season={season} value={state.conditions} onChange={(conditions) => update({ conditions })} officialQuali={round.has_quali && !round.has_results} officialGrid={round.has_grid && !round.has_results} />
        </Card>
      </div>
    </div>
  );
}

function TeamLine({ team, byId, projById, chip }: { team: TeamView; byId: Map<string, { tla?: string; team_name: string; name: string }>; projById: Map<string, { mean: number }>; chip: string }) {
  const all = [...team.drivers, ...team.constructors];
  return (
    <div>
      <div className="mb-2 text-xs uppercase tracking-[0.12em] text-ink-3">Line-up</div>
      <div className="flex flex-wrap gap-2">
        {all.map((a) => {
          const meta = byId.get(a.id);
          const star = a.id === team.captain_id ? (chip === "3x" ? " ×3" : " ×2") : a.id === team.boost_id ? " ×2" : "";
          return (
            <span key={a.id} className={`chip flex items-center px-2 py-1 text-sm ${star ? "border-accent/60" : ""}`}>
              <TeamEdge team={meta?.team_name ?? ""} />
              {a.kind === "driver" ? meta?.tla ?? a.name : a.name}
              {star && <span className="ml-1 text-xs font-semibold text-accent">{star}</span>}
              <span className="num ml-2 text-xs text-ink-3">{projById.get(a.id)?.mean.toFixed(1)}</span>
            </span>
          );
        })}
      </div>
    </div>
  );
}

function rangeOf(team: TeamView, proj: Map<string, { p10: number; p50: number; p90: number; mean: number }>) {
  let p10 = 0,
    p50 = 0,
    p90 = 0;
  for (const a of [...team.drivers, ...team.constructors]) {
    const p = proj.get(a.id);
    if (!p) continue;
    const mult = a.id === team.captain_id ? (team.boost_id ? 3 : 2) : a.id === team.boost_id ? 2 : 1;
    p10 += p.p10 * mult;
    p50 += p.p50 * mult;
    p90 += p.p90 * mult;
  }
  return { p10: p10 + team.penalty, p50: p50 + team.penalty, p90: p90 + team.penalty };
}
