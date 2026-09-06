// Backend selection. The local binary serves a JSON API; the static site
// runs the same engine as WebAssembly inside a Web Worker. Both expose the
// same typed interface.

import type { BacktestReport, Conditions, HindsightView, OptimizeInput, OptimizeView, PricesView, ProjectionView, ReviewView, RulesView, SeasonView } from "./api";

export interface ReviewInput {
  round: number;
  sims?: number;
  team?: string[];
  captain?: string;
}

export interface Backend {
  readonly kind: "api" | "wasm";
  season(): Promise<SeasonView>;
  rules(): Promise<RulesView>;
  project(round: number, sims: number, cond: Conditions): Promise<ProjectionView>;
  optimize(input: OptimizeInput): Promise<OptimizeView>;
  prices(): Promise<PricesView>;
  backtest(sims?: number): Promise<BacktestReport>;
  hindsight(round: number, top?: number): Promise<HindsightView>;
  review(input: ReviewInput): Promise<ReviewView>;
  sync?(): Promise<SeasonView>;
}

// --- HTTP API ---------------------------------------------------------------

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function post<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
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

export const apiBackend: Backend = {
  kind: "api",
  season: () => get("/api/season"),
  rules: () => get("/api/rules"),
  project: (round, sims, cond) => get(`/api/project?round=${round}&sims=${sims}&${condQuery(cond)}`),
  optimize: (input) => post("/api/optimize", input),
  prices: () => get("/api/prices"),
  backtest: (sims = 3000) => get(`/api/backtest?sims=${sims}`),
  hindsight: (round, top = 5) => get(`/api/hindsight?round=${round}&top=${top}`),
  review: (input) => post("/api/review", input),
  sync: () => post("/api/sync", {}),
};

// --- WebAssembly worker -----------------------------------------------------

class WasmBackend implements Backend {
  readonly kind = "wasm" as const;
  private worker = new Worker(new URL("./worker.ts", import.meta.url), { type: "classic" });
  private next = 1;
  private pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();

  constructor() {
    this.worker.onmessage = (ev: MessageEvent<{ id: number; result?: unknown; error?: string }>) => {
      const p = this.pending.get(ev.data.id);
      if (!p) return;
      this.pending.delete(ev.data.id);
      if (ev.data.error !== undefined) p.reject(new Error(ev.data.error));
      else p.resolve(ev.data.result);
    };
  }

  private call<T>(method: string, ...args: unknown[]): Promise<T> {
    const id = this.next++;
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (v: unknown) => void, reject });
      this.worker.postMessage({ id, method, args });
    });
  }

  season = () => this.call<SeasonView>("season");
  rules = () => this.call<RulesView>("rules");
  project = (round: number, sims: number, cond: Conditions) => this.call<ProjectionView>("project", JSON.stringify({ round, sims, conditions: cond }));
  optimize = (input: OptimizeInput) => this.call<OptimizeView>("optimize", JSON.stringify(input));
  prices = () => this.call<PricesView>("prices");
  backtest = (sims = 3000) => this.call<BacktestReport>("backtest", sims);
  hindsight = (round: number, top = 5) => this.call<HindsightView>("hindsight", round, top);
  review = (input: ReviewInput) => this.call<ReviewView>("review", JSON.stringify(input));
}

export const isStatic = import.meta.env.VITE_STATIC === "1";

export const backend: Backend = isStatic ? new WasmBackend() : apiBackend;
