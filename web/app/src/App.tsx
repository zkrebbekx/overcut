import { useEffect, useMemo, useState } from "react";
import { Activity, BarChart3, BookOpen, Crosshair, History, RefreshCw, TrendingUp } from "lucide-react";
import { api, isStatic, type Conditions, type SeasonView } from "./api";
import { usePlayerState, type PlayerState } from "./store";
import { DecideView } from "./views/Decide";
import { ProjectionsView } from "./views/Projections";
import { PricesView } from "./views/Prices";
import { TrustView } from "./views/Trust";
import { HindsightView } from "./views/Hindsight";
import { RulesView } from "./views/Rules";
import { ErrorBox, Spinner } from "./components/ui";
import { ErrorBoundary } from "./components/ErrorBoundary";

type Tab = "decide" | "projections" | "prices" | "trust" | "hindsight" | "rules";

const tabs: { id: Tab; label: string; icon: typeof Crosshair }[] = [
  { id: "decide", label: "Decide", icon: Crosshair },
  { id: "projections", label: "Projections", icon: BarChart3 },
  { id: "prices", label: "Prices", icon: TrendingUp },
  { id: "trust", label: "Trust", icon: Activity },
  { id: "hindsight", label: "Hindsight", icon: History },
  { id: "rules", label: "Rules", icon: BookOpen },
];

export default function App() {
  const [tab, setTab] = useState<Tab>(() => (location.hash.replace("#", "") as Tab) || "decide");
  const [season, setSeason] = useState<SeasonView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [state, update] = usePlayerState();

  useEffect(() => {
    api
      .season()
      .then((s) => {
        setSeason(s);
        const fromUrl = teamFromURL(s);
        if (fromUrl) update(fromUrl);
      })
      .catch((e) => setError(String(e.message ?? e)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (location.hash.replace("#", "") !== tab) location.hash = tab;
  }, [tab]);

  // Follow the browser's back and forward buttons and external links.
  useEffect(() => {
    const onHash = () => {
      const next = location.hash.replace("#", "") as Tab;
      if (tabs.some((t) => t.id === next)) setTab(next);
    };
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const round = useMemo(() => {
    if (!season) return null;
    const n = state.round || season.next_round;
    return season.rounds.find((r) => r.round === n) ?? null;
  }, [season, state.round]);

  const knows = describeKnowledge(state.conditions, !!round?.has_quali, !!round?.has_grid, !!round?.has_results);

  async function sync() {
    if (!api.sync) return;
    setSyncing(true);
    try {
      setSeason(await api.sync());
    } catch (e) {
      setError(String((e as Error).message ?? e));
    } finally {
      setSyncing(false);
    }
  }

  return (
    <div className="flex min-h-screen">
      <nav aria-label="Primary" className="hidden w-52 shrink-0 flex-col border-r border-line bg-panel p-3 md:flex">
        <div className="display mb-6 px-2 pt-1 text-lg font-bold tracking-tight text-ink">
          <span className="text-accent">over</span>cut
        </div>
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            aria-current={tab === t.id ? "page" : undefined}
            className={`mb-1 flex items-center gap-3 rounded-[8px] px-3 py-2 text-sm transition ${tab === t.id ? "bg-raised text-ink" : "text-ink-2 hover:bg-raised/60 hover:text-ink"}`}
          >
            <t.icon size={16} className={tab === t.id ? "text-accent" : ""} />
            {t.label}
          </button>
        ))}
        <div className="mt-auto px-2 text-[11px] leading-relaxed text-ink-3">
          {season && (
            <>
              Season {season.season}
              <br />
              Synced {new Date(season.synced_at).toLocaleString()}
            </>
          )}
        </div>
      </nav>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-10 flex flex-wrap items-center gap-3 border-b border-line bg-bg/90 px-4 py-3 backdrop-blur md:px-6">
          <div className="display text-lg font-bold md:hidden">
            <span className="text-accent">over</span>cut
          </div>
          {season && (
            <select
              aria-label="Round"
              value={state.round || season.next_round}
              onChange={(e) => update({ round: Number(e.target.value) })}
              className="chip px-2 py-1 text-sm text-ink"
            >
              {season.rounds.map((r) => (
                <option key={r.round} value={r.round}>
                  R{r.round} · {r.name}
                  {r.has_sprint ? " · Sprint" : ""}
                  {r.has_results ? " ✓" : ""}
                </option>
              ))}
            </select>
          )}
          <span className={`chip px-2 py-1 text-xs ${knows.tone === "warn" ? "text-warn" : "text-gain"}`} title={knows.detail}>
            {knows.label}
          </span>
          {isStatic ? (
            <span className="ml-auto text-xs text-ink-3" title="Runs entirely in your browser. Data refreshes automatically after each session.">
              {season ? `Data ${new Date(season.synced_at).toLocaleDateString()}` : ""}
            </span>
          ) : (
            <button onClick={sync} disabled={syncing} className="chip ml-auto flex items-center gap-2 px-3 py-1 text-xs text-ink-2 hover:text-ink disabled:opacity-50">
              <RefreshCw size={12} className={syncing ? "animate-spin" : ""} />
              {syncing ? "Syncing…" : "Sync data"}
            </button>
          )}
        </header>

        <main className="min-w-0 flex-1 overflow-x-hidden p-4 pb-24 md:p-6 md:pb-6">
          {error && <ErrorBox error={error} />}
          {!season && !error && <Spinner label={isStatic ? "Starting the engine in your browser…" : "Loading season…"} />}
          {season && round && (
            <ErrorBoundary resetKey={tab}>
              <div key={tab} className="fade-in">
                {tab === "decide" && <DecideView season={season} round={round} state={state} update={update} />}
                {tab === "projections" && <ProjectionsView season={season} round={round} state={state} />}
                {tab === "prices" && <PricesView />}
                {tab === "trust" && <TrustView />}
                {tab === "hindsight" && <HindsightView season={season} />}
                {tab === "rules" && <RulesView />}
              </div>
            </ErrorBoundary>
          )}
        </main>
      </div>

      <nav aria-label="Primary" className="fixed inset-x-0 bottom-0 z-10 flex border-t border-line bg-panel md:hidden">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            aria-current={tab === t.id ? "page" : undefined}
            className={`flex flex-1 flex-col items-center gap-1 py-2 text-[10px] ${tab === t.id ? "text-accent" : "text-ink-3"}`}
          >
            <t.icon size={18} />
            {t.label}
          </button>
        ))}
      </nav>
    </div>
  );
}

// A shareable link can carry the team and settings:
//   /?team=ANT,HUL,BOR,COL,LIN,Mercedes,McLaren&free=3&cash=1.6&back=ANT,ALB
// Driver codes and constructor-name prefixes resolve against the season.
function teamFromURL(season: SeasonView): Partial<PlayerState> | null {
  const q = new URLSearchParams(location.search);
  if (![...q.keys()].length) return null;
  const patch: Partial<PlayerState> = {};
  const team = q.get("team");
  if (team) {
    const ids: string[] = [];
    for (const tok of team.split(",")) {
      const t = tok.trim().toLowerCase();
      const hit = season.assets.find((a) => a.selectable && (a.tla?.toLowerCase() === t || a.name.toLowerCase().startsWith(t)));
      if (hit) ids.push(hit.id);
    }
    patch.team = ids;
  }
  if (q.get("free")) patch.freeTransfers = Number(q.get("free"));
  if (q.get("cash")) patch.cash = Number(q.get("cash"));
  const cond: Conditions = {};
  for (const k of ["quali", "grid", "back", "fp3"] as const) {
    const v = q.get(k);
    if (v) cond[k] = v.split(",").map((s) => s.trim().toUpperCase());
  }
  if (Object.keys(cond).length) patch.conditions = cond;
  history.replaceState(null, "", location.pathname + location.hash);
  return patch;
}

function describeKnowledge(c: { quali?: string[]; grid?: string[]; back?: string[]; fp3?: string[] }, officialQuali: boolean, officialGrid: boolean, complete: boolean) {
  if (complete && !c.grid?.length && !c.quali?.length) {
    return { label: "Round complete · official grid", tone: "ok", detail: "A finished round: the projection uses the real qualifying and grid, so you can compare it with the actual points." };
  }
  const parts: string[] = [];
  if (c.grid?.length) parts.push("grid");
  else if (officialGrid) parts.push("official grid with penalties");
  else if (c.quali?.length) parts.push("qualifying");
  else if (officialQuali) parts.push("official qualifying");
  if (c.back?.length && !officialGrid && !c.grid?.length) parts.push("penalties");
  if (c.fp3?.length) parts.push("FP3");
  if (parts.length === 0) {
    return { label: "Before qualifying", tone: "warn", detail: "Projections sample the grid from form." };
  }
  return { label: `Model knows: ${parts.join(", ")}`, tone: "ok", detail: "Projections use the known weekend state." };
}
