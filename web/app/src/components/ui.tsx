// Small shared building blocks.

import type { ReactNode } from "react";

export function Card({ title, right, children, className = "" }: { title?: ReactNode; right?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`card p-4 ${className}`}>
      {(title || right) && (
        <header className="mb-3 flex items-center justify-between gap-3">
          <h2 className="display text-xs font-semibold uppercase tracking-[0.14em] text-ink-2">{title}</h2>
          {right}
        </header>
      )}
      {children}
    </section>
  );
}

export function Stat({ label, value, sub, tone }: { label: string; value: ReactNode; sub?: ReactNode; tone?: "gain" | "loss" | "accent" }) {
  const color = tone === "gain" ? "text-gain" : tone === "loss" ? "text-loss" : tone === "accent" ? "text-accent" : "text-ink";
  return (
    <div className="card p-4">
      <div className="text-xs uppercase tracking-[0.12em] text-ink-3">{label}</div>
      <div className={`display num mt-1 text-2xl font-semibold ${color}`}>{value}</div>
      {sub && <div className="mt-1 text-xs text-ink-2">{sub}</div>}
    </div>
  );
}

export function Delta({ value, digits = 1, suffix = "" }: { value: number; digits?: number; suffix?: string }) {
  const tone = value > 0 ? "text-gain" : value < 0 ? "text-loss" : "text-ink-2";
  const arrow = value > 0 ? "▲" : value < 0 ? "▼" : "—";
  return (
    <span className={`num ${tone}`}>
      {arrow} {value > 0 ? "+" : ""}
      {value.toFixed(digits)}
      {suffix}
    </span>
  );
}

export function Segmented<T extends string>({ value, options, onChange }: { value: T; options: { value: T; label: string }[]; onChange: (v: T) => void }) {
  return (
    <div role="radiogroup" className="chip inline-flex overflow-hidden p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          role="radio"
          aria-checked={value === o.value}
          onClick={() => onChange(o.value)}
          className={`rounded-[4px] px-3 py-1 text-xs font-medium transition ${value === o.value ? "bg-accent text-bg" : "text-ink-2 hover:text-ink"}`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Spinner({ label }: { label: string }) {
  return (
    <div role="status" className="flex items-center gap-3 text-sm text-ink-2">
      <span className="inline-block h-3 w-3 animate-spin rounded-full border-2 border-accent border-t-transparent" />
      {label}
    </div>
  );
}

export function ErrorBox({ error }: { error: string }) {
  return (
    <div role="alert" className="rounded-[10px] border border-loss/40 bg-loss/10 p-3 text-sm text-loss">
      {error}
    </div>
  );
}

export function money(v: number) {
  return `$${v.toFixed(1)}M`;
}

export function TeamEdge({ teamId }: { teamId: string }) {
  return <span aria-hidden className="mr-2 inline-block h-4 w-[3px] rounded-sm" style={{ background: teamColor(teamId) }} />;
}

// A neutral, evenly spaced hue per team id, used only as a 3px edge.
export function teamColor(teamId: string) {
  let h = 0;
  for (const ch of teamId) h = (h * 31 + ch.charCodeAt(0)) % 360;
  return `oklch(0.72 0.14 ${h})`;
}
