// What the model knows about the weekend. Each field takes a
// comma-separated list of driver codes (TLAs) from P1.

import type { Conditions } from "../api";
import type { SeasonView } from "../api";

function parse(s: string): string[] {
  return s
    .split(/[,\s]+/)
    .map((t) => t.trim().toUpperCase())
    .filter(Boolean);
}

export function ConditionsPanel({ season, value, onChange, officialQuali, officialGrid }: { season: SeasonView; value: Conditions; onChange: (c: Conditions) => void; officialQuali?: boolean; officialGrid?: boolean }) {
  const known = new Set(season.assets.filter((a) => a.kind === "driver" && a.selectable).map((a) => a.tla));
  const field = (key: keyof Conditions, label: string, hint: string) => {
    const text = (value[key] ?? []).join(", ");
    const bad = (value[key] ?? []).filter((t) => !known.has(t));
    return (
      <label className="flex flex-col gap-1 text-xs text-ink-3">
        <span className="flex items-center justify-between">
          {label}
          {bad.length > 0 && <span className="text-warn">unknown: {bad.join(", ")}</span>}
        </span>
        <input
          defaultValue={text}
          onBlur={(e) => onChange({ ...value, [key]: parse(e.target.value) })}
          placeholder={hint}
          className="chip num px-2 py-1.5 text-sm text-ink placeholder:text-ink-3"
        />
      </label>
    );
  };
  return (
    <div className="space-y-3">
      <p className="text-xs leading-relaxed text-ink-3">
        Enter driver codes from P1, comma-separated. Leave blank for anything not yet known; the model samples it from form.
      </p>
      {officialGrid && !(value.grid?.length ?? 0) ? (
        <p className="rounded-[6px] border border-gain/30 bg-gain/10 px-2 py-1.5 text-xs text-gain">Official starting grid loaded, with every penalty applied. Enter a grid below only to override it.</p>
      ) : officialQuali && !(value.quali?.length ?? 0) ? (
        <p className="rounded-[6px] border border-gain/30 bg-gain/10 px-2 py-1.5 text-xs text-gain">Official qualifying result loaded. The starting grid follows once the official grid is published; add known penalties under "Back of grid" until then.</p>
      ) : null}
      {field("quali", "Qualifying order", officialQuali ? "official result in use" : "RUS, HAM, VER, …")}
      {field("grid", "Starting grid (after penalties)", officialGrid ? "official grid in use" : "leave blank to derive from qualifying")}
      {field("back", "Back of grid", officialGrid ? "not needed: official grid in use" : "ANT, ALB")}
      {field("fp3", "FP3 order (pace prior)", "RUS, HAM, VER, …")}
      <button onClick={() => onChange({})} className="text-xs text-ink-3 underline-offset-2 hover:text-ink hover:underline">
        Clear weekend state
      </button>
    </div>
  );
}
