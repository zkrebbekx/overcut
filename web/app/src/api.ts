// Typed client for the local overcut API.

export type Kind = "driver" | "constructor";

export interface RoundView {
  round: number;
  name: string;
  circuit_id: string;
  date: string;
  has_sprint: boolean;
  has_results: boolean;
}

export interface HistoryView {
  gameday: number;
  price: number;
  points: number;
  quali_pts: number;
  sprint_pts: number;
  race_pts: number;
  ownership: number;
}

export interface AssetView {
  id: string;
  kind: Kind;
  name: string;
  tla?: string;
  team_id: string;
  team_name: string;
  price: number;
  old_price: number;
  ownership: number;
  total_points: number;
  selectable: boolean;
  history: HistoryView[];
}

export interface SeasonView {
  season: number;
  synced_at: string;
  latest_gameday: number;
  next_round: number;
  rounds: RoundView[];
  assets: AssetView[];
}

export interface Conditions {
  quali?: string[];
  grid?: string[];
  back?: string[];
  fp3?: string[];
}

export interface AssetProjection {
  id: string;
  name: string;
  kind: Kind;
  tla?: string;
  team_id: string;
  price: number;
  ownership: number;
  mean: number;
  sd: number;
  p10: number;
  p50: number;
  p90: number;
  last_points: number;
  avg_points: number;
}

export interface ProjectionView {
  round: number;
  name: string;
  has_sprint: boolean;
  sims: number;
  conditions: Conditions;
  assets: AssetProjection[];
}

export interface TeamAsset {
  id: string;
  name: string;
  kind: Kind;
  price: number;
  points: number;
}

export interface TeamView {
  drivers: TeamAsset[];
  constructors: TeamAsset[];
  captain_id: string;
  boost_id?: string;
  cost: number;
  raw_points: number;
  captain_points: number;
  transfers: number;
  penalty: number;
  score: number;
  in: string[];
  out: string[];
}

export interface OptimizeInput {
  round?: number;
  sims?: number;
  seed?: number;
  team: string[];
  free_transfers: number;
  budget?: number;
  chip?: "" | "wildcard" | "limitless" | "3x";
  risk?: "mean" | "p10" | "p90";
  top?: number;
  conditions?: Conditions;
}

export interface OptimizeView {
  round: number;
  name: string;
  risk: string;
  chip: string;
  budget: number;
  teams: TeamView[];
  current_score: number;
  projection: ProjectionView;
}

export interface PricePrediction {
  AssetID: string;
  Name: string;
  Kind: Kind;
  Price: number;
  Change: number;
}

export interface PricesView {
  report: {
    Examples: number;
    MAE: number;
    NaiveMAE: number;
    Direction: number;
    Moves: number;
  };
  predictions: PricePrediction[];
}

export interface BacktestRound {
  Round: number;
  Name: string;
  DriverMAE: number;
  ConsMAE: number;
  SpearmanRho: number;
  ModelTeamPts: number;
  NaiveTeamPts: number;
  HindsightTeamPts: number;
}

export interface BacktestReport {
  Rounds: BacktestRound[];
  DriverMAE: number;
  ConsMAE: number;
  MeanSpearman: number;
  BaselinePrev: number;
  BaselineSeason: number;
  ModelTeamPts: number;
  NaiveTeamPts: number;
  HindsightTeamPts: number;
}

export interface HindsightView {
  round: number;
  name: string;
  teams: TeamView[];
}

export type RulesView = Record<string, unknown>;

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function post<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error ?? `${res.status} ${res.statusText}`);
  }
  return res.json();
}

function condQuery(c: Conditions): string {
  const p = new URLSearchParams();
  if (c.quali?.length) p.set("quali", c.quali.join(","));
  if (c.grid?.length) p.set("grid", c.grid.join(","));
  if (c.back?.length) p.set("back", c.back.join(","));
  if (c.fp3?.length) p.set("fp3", c.fp3.join(","));
  return p.toString();
}

export const api = {
  season: () => get<SeasonView>("/api/season"),
  rules: () => get<RulesView>("/api/rules"),
  project: (round: number, sims: number, cond: Conditions) =>
    get<ProjectionView>(`/api/project?round=${round}&sims=${sims}&${condQuery(cond)}`),
  optimize: (input: OptimizeInput) => post<OptimizeView>("/api/optimize", input),
  prices: () => get<PricesView>("/api/prices"),
  backtest: (sims = 3000) => get<BacktestReport>(`/api/backtest?sims=${sims}`),
  hindsight: (round: number, top = 5) => get<HindsightView>(`/api/hindsight?round=${round}&top=${top}`),
  sync: () => post<SeasonView>("/api/sync", {}),
};
