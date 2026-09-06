// The best possible team for a finished round, by official points.

import { useEffect, useState } from "react";
import { api, type HindsightView as Hindsight, type SeasonView } from "../api";
import { Card, ErrorBox, Spinner, TeamEdge, money } from "../components/ui";

export function HindsightView({ season }: { season: SeasonView }) {
  const done = season.rounds.filter((r) => r.has_results);
  const [round, setRound] = useState(done[done.length - 1]?.round ?? 1);
  const [data, setData] = useState<Hindsight | null>(null);
  const [error, setError] = useState<string | null>(null);
  const byId = new Map(season.assets.map((a) => [a.id, a]));

  useEffect(() => {
    setData(null);
    api.hindsight(round, 8).then(setData).catch((e) => setError(String(e.message ?? e)));
  }, [round]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <select aria-label="Round" value={round} onChange={(e) => setRound(Number(e.target.value))} className="chip px-2 py-1 text-sm text-ink">
          {done.map((r) => (
            <option key={r.round} value={r.round}>
              R{r.round} · {r.name}
            </option>
          ))}
        </select>
        <span className="text-xs text-ink-3">Within the $100M cap, at that round's prices, Boost on the top scorer.</span>
      </div>
      {error && <ErrorBox error={error} />}
      {!data && !error && <Spinner label="Enumerating every legal team…" />}
      {data && (
        <div className="grid gap-3 lg:grid-cols-2">
          {data.teams.map((t, i) => (
            <Card key={i} title={i === 0 ? <span className="text-accent">#1 · best possible</span> : `#${i + 1}`} right={<span className={`num text-sm font-semibold ${i === 0 ? "text-accent" : "text-ink"}`}>{t.score.toFixed(0)} pts · {money(t.cost)}</span>}>
              <ul className="space-y-1 text-sm">
                {[...t.drivers, ...t.constructors].map((a) => (
                  <li key={a.id} className="flex items-center justify-between">
                    <span className="flex items-center text-ink-2">
                      <TeamEdge team={byId.get(a.id)?.team_name ?? ""} />
                      {a.name}
                      {a.id === t.captain_id && <span className="ml-2 text-xs font-semibold text-accent">×2</span>}
                    </span>
                    <span className="num text-ink">{a.points.toFixed(0)}</span>
                  </li>
                ))}
              </ul>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
