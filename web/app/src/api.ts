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
  team_name: string;
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

// The concrete backend (HTTP API or in-browser WebAssembly) is chosen at
// build time; see backend.ts.
export { backend as api, isStatic } from "./backend";
