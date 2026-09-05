// Small shared building blocks.

import { useEffect, useState, type ReactNode } from "react";

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

// A numeric field that edits comfortably: the text buffer is local, so an
// empty field stays empty while typing; the parent gets every valid value;
// blur tidies the text. Focus selects all so a tap replaces the value.
export function NumberField({ value, onChange, step = 1, min, className = "", ariaLabel }: { value: number; onChange: (v: number) => void; step?: number; min?: number; className?: string; ariaLabel?: string }) {
  const [text, setText] = useState(String(value));
  const [editing, setEditing] = useState(false);
  useEffect(() => {
    if (!editing) setText(String(value));
  }, [value, editing]);
  return (
    <input
      type="text"
      inputMode="decimal"
      aria-label={ariaLabel}
      value={text}
      onFocus={(e) => {
        setEditing(true);
        const el = e.target;
        requestAnimationFrame(() => el.select());
      }}
      onChange={(e) => {
        const t = e.target.value;
        setEditing(true);
        setText(t);
        const n = Number(t);
        if (t.trim() !== "" && Number.isFinite(n) && (min === undefined || n >= min)) onChange(n);
      }}
      onBlur={() => {
        setEditing(false);
        const n = Number(text);
        const ok = text.trim() !== "" && Number.isFinite(n) && (min === undefined || n >= min);
        const v = ok ? n : (min ?? 0);
        onChange(v);
        setText(String(v));
      }}
      onKeyDown={(e) => {
        if (e.key === "ArrowUp" || e.key === "ArrowDown") {
          e.preventDefault();
          const dir = e.key === "ArrowUp" ? 1 : -1;
          const n = Math.max(min ?? -Infinity, Math.round((value + dir * step) * 1000) / 1000);
          onChange(n);
          setText(String(n));
        }
      }}
      className={`chip num px-2 py-1 text-sm text-ink ${className}`}
    />
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

// A 3px edge in the team's livery colour. Team colour is a data accent
// only; it never colours chrome or text.
export function TeamEdge({ team }: { team: string }) {
  return <span aria-hidden title={team} className="mr-2 inline-block h-4 w-[3px] rounded-sm" style={{ background: teamColor(team) }} />;
}

const liveries: [RegExp, string][] = [
  [/mercedes/i, "#27f4d2"],
  [/ferrari/i, "#e8002d"],
  [/mclaren/i, "#ff8000"],
  [/red bull/i, "#3671c6"],
  [/racing bulls|rb\b/i, "#6692ff"],
  [/alpine/i, "#ff87bc"],
  [/aston/i, "#229971"],
  [/williams/i, "#64c4ff"],
  [/haas/i, "#b6babd"],
  [/audi|sauber/i, "#f50537"],
  [/cadillac/i, "#d4af37"],
];

export function teamColor(team: string) {
  for (const [re, hex] of liveries) if (re.test(team)) return hex;
  let h = 0;
  for (const ch of team) h = (h * 31 + ch.charCodeAt(0)) % 360;
  return `oklch(0.72 0.14 ${h})`;
}
