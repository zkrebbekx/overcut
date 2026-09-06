// A P10–P90 range on a shared axis, with the mean as a tick and P50 as a
// dot. Every instance in a table shares min/max so bars compare.

export function RangeBar({ p10, p50, p90, mean, min, max }: { p10: number; p50: number; p90: number; mean: number; min: number; max: number }) {
  const span = max - min || 1;
  const x = (v: number) => `${((v - min) / span) * 100}%`;
  const zero = min < 0 && max > 0 ? x(0) : null;
  return (
    <div
      className="relative h-5 w-full"
      role="img"
      aria-label={`P10 ${p10.toFixed(0)}, median ${p50.toFixed(0)}, mean ${mean.toFixed(1)}, P90 ${p90.toFixed(0)}`}
    >
      {zero && <div className="absolute top-0 h-full w-px bg-line" style={{ left: zero }} />}
      <div className="absolute top-[7px] h-[6px] rounded-full bg-mark-dim" style={{ left: x(p10), width: `calc(${x(p90)} - ${x(p10)})` }} />
      <div className="absolute top-[8px] h-[4px] w-[4px] rounded-full bg-ink" style={{ left: `calc(${x(p50)} - 2px)` }} />
      <div className="absolute top-[3px] h-[14px] w-[2px] rounded-sm bg-mark" style={{ left: `calc(${x(mean)} - 1px)` }} />
    </div>
  );
}
