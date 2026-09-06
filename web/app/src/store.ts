// Browser-persisted player state: team, transfers, chip, risk, weekend
// conditions. Everything lives in localStorage under one key.

import { useCallback, useEffect, useState } from "react";
import type { Conditions } from "./api";

export type Risk = "mean" | "p10" | "p90";
export type Chip = "" | "wildcard" | "limitless" | "3x" | "nonegative";

export interface PlayerState {
  team: string[]; // asset IDs
  freeTransfers: number;
  cash: number; // remaining cost cap in millions
  chip: Chip;
  risk: Risk;
  round: number; // 0 = next
  sims: number;
  conditions: Conditions;
}

const KEY = "overcut.player.v1";

export const defaultState: PlayerState = {
  team: [],
  freeTransfers: 2,
  cash: 0,
  chip: "",
  risk: "mean",
  round: 0,
  sims: 20000,
  conditions: {},
};

function load(): PlayerState {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return defaultState;
    return { ...defaultState, ...JSON.parse(raw) };
  } catch {
    return defaultState;
  }
}

export function usePlayerState(): [PlayerState, (patch: Partial<PlayerState>) => void] {
  const [state, setState] = useState<PlayerState>(load);
  useEffect(() => {
    localStorage.setItem(KEY, JSON.stringify(state));
  }, [state]);
  const update = useCallback((patch: Partial<PlayerState>) => {
    setState((s) => ({ ...s, ...patch }));
  }, []);
  return [state, update];
}
