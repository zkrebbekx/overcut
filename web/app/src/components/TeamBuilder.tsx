// The current-team editor: five drivers and two constructors, chosen from
// searchable chips, plus transfers, cash, chip, and risk.

import { useMemo, useState } from "react";
import { X } from "lucide-react";
import type { AssetView, SeasonView } from "../api";
import type { Chip, PlayerState, Risk } from "../store";
import { Segmented, TeamEdge, money } from "./ui";

export function TeamBuilder({ season, state, update }: { season: SeasonView; state: PlayerState; update: (p: Partial<PlayerState>) => void }) {
  const [query, setQuery] = useState("");
  const byId = useMemo(() => new Map(season.assets.map((a) => [a.id, a])), [season]);
  const team = state.team.map((id) => byId.get(id)).filter((a): a is AssetView => !!a);
  const drivers = team.filter((a) => a.kind === "driver");
  const cons = team.filter((a) => a.kind === "constructor");
  const value = team.reduce((s, a) => s + a.price, 0);

  const candidates = useMemo(() => {
    const q = query.trim().toLowerCase();
    return season.assets
      .filter((a) => a.selectable && !state.team.includes(a.id))
      .filter((a) => (a.kind === "driver" ? drivers.length < 5 : cons.length < 2))
      .filter((a) => !q || a.name.toLowerCase().includes(q) || a.tla?.toLowerCase().includes(q) || a.team_name.toLowerCase().includes(q))
      .sort((a, b) => b.price - a.price);
  }, [season, state.team, query, drivers.length, cons.length]);

  const complete = drivers.length === 5 && cons.length === 2;

  return (
    <div className="space-y-4">
      <div>
        <div className="mb-2 flex items-center justify-between text-xs text-ink-3">
          <span>
            {drivers.length}/5 drivers · {cons.length}/2 constructors
          </span>
          <span className="num">
            value {money(value)} + cash {money(state.cash)} = <span className="text-ink">{money(value + state.cash)}</span>
          </span>
        </div>
        <div className="flex flex-wrap gap-2">
          {team.map((a) => (
            <span key={a.id} className="chip flex items-center py-1 pl-2 pr-1 text-sm">
              <TeamEdge team={a.team_name} />
              {a.kind === "driver" ? a.tla : a.name}
              <span className="num ml-2 text-xs text-ink-3">{a.price.toFixed(1)}</span>
              <button aria-label={`Remove ${a.name}`} onClick={() => update({ team: state.team.filter((id) => id !== a.id) })} className="ml-1 rounded p-0.5 text-ink-3 hover:text-loss">
                <X size={14} />
              </button>
            </span>
          ))}
          {team.length === 0 && <span className="text-sm text-ink-3">Add five drivers and two constructors to see your move.</span>}
        </div>
      </div>

      {!complete && (
        <div>
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search driver or team…"
            aria-label="Search assets"
            className="chip w-full px-3 py-2 text-sm text-ink placeholder:text-ink-3"
          />
          <div className="mt-2 flex max-h-40 flex-wrap gap-1.5 overflow-y-auto">
            {candidates.map((a) => (
              <button key={a.id} onClick={() => update({ team: [...state.team, a.id] })} className="chip flex items-center px-2 py-1 text-xs text-ink-2 hover:border-accent hover:text-ink">
                <TeamEdge team={a.team_name} />
                {a.kind === "driver" ? `${a.tla} · ${a.name.split(" ").slice(-1)}` : a.name}
                <span className="num ml-2 text-ink-3">{a.price.toFixed(1)}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 text-xs">
        <label className="flex flex-col gap-1 text-ink-3">
          Free transfers
          <input type="number" min={0} max={9} value={state.freeTransfers} onChange={(e) => update({ freeTransfers: Number(e.target.value) })} className="chip num px-2 py-1 text-sm text-ink" />
        </label>
        <label className="flex flex-col gap-1 text-ink-3">
          Cash in hand ($M)
          <input type="number" step={0.1} value={state.cash} onChange={(e) => update({ cash: Number(e.target.value) })} className="chip num px-2 py-1 text-sm text-ink" />
        </label>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-xs text-ink-3">
        <span>Chip</span>
        <Segmented<Chip>
          value={state.chip}
          onChange={(chip) => update({ chip })}
          options={[
            { value: "", label: "None" },
            { value: "wildcard", label: "Wildcard" },
            { value: "limitless", label: "Limitless" },
            { value: "3x", label: "x3 Boost" },
          ]}
        />
      </div>
      <div className="flex flex-wrap items-center gap-3 text-xs text-ink-3">
        <span>Optimize for</span>
        <Segmented<Risk>
          value={state.risk}
          onChange={(risk) => update({ risk })}
          options={[
            { value: "mean", label: "Expected" },
            { value: "p10", label: "Safe floor" },
            { value: "p90", label: "Ceiling" },
          ]}
        />
      </div>
    </div>
  );
}
