
import { cn } from "@/lib/utils";

type NumberOptionRangeSliderProps = {
  min: number;
  max: number;
  step?: number;
  from: number;
  to: number;
  className?: string;
  /** When false, omit domain labels under the track. */
  showBounds?: boolean;
  onChange: (next: { from: number; to: number }) => void;
};

export function formatOptionNumber(value: number, step?: number): string {
  if (step != null && step > 0 && step < 1) {
    const digits = Math.min(6, Math.max(0, Math.ceil(-Math.log10(step))));
    return value.toFixed(digits);
  }
  if (Number.isInteger(value)) return String(value);
  return String(Number(value.toPrecision(6)));
}

function pct(value: number, min: number, max: number): number {
  if (max <= min) return 0;
  return ((value - min) / (max - min)) * 100;
}

/**
 * Dual-thumb range slider. Track ends map to option min/max;
 * thumbs stay in-domain with from ≤ to.
 */
export function NumberOptionRangeSlider({
  min,
  max,
  step = 1,
  from,
  to,
  className,
  showBounds = true,
  onChange,
}: NumberOptionRangeSliderProps) {
  const safeStep = step > 0 ? step : 1;
  const lo = Math.min(from, to);
  const hi = Math.max(from, to);
  const leftPct = pct(lo, min, max);
  const rightPct = pct(hi, min, max);
  const fromOnTop = lo > min + (max - min) * 0.5;

  function setFrom(raw: number) {
    const next = Math.min(Math.max(raw, min), max);
    onChange({ from: Math.min(next, hi), to: hi });
  }

  function setTo(raw: number) {
    const next = Math.min(Math.max(raw, min), max);
    onChange({ from: lo, to: Math.max(next, lo) });
  }

  const thumbClass =
    "pointer-events-none absolute inset-0 h-full w-full appearance-none bg-transparent " +
    "[&::-webkit-slider-runnable-track]:h-1.5 [&::-webkit-slider-runnable-track]:bg-transparent " +
    "[&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:relative " +
    "[&::-webkit-slider-thumb]:mt-[-5px] [&::-webkit-slider-thumb]:h-3.5 [&::-webkit-slider-thumb]:w-3.5 " +
    "[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full " +
    "[&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-background " +
    "[&::-webkit-slider-thumb]:bg-foreground [&::-webkit-slider-thumb]:shadow " +
    "[&::-moz-range-track]:h-1.5 [&::-moz-range-track]:bg-transparent " +
    "[&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:h-3.5 [&::-moz-range-thumb]:w-3.5 " +
    "[&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-2 " +
    "[&::-moz-range-thumb]:border-background [&::-moz-range-thumb]:bg-foreground " +
    "[&::-moz-range-thumb]:border-none";

  return (
    <div className={cn(showBounds ? "grid gap-1" : undefined, className)}>
      <div className="relative h-7">
        <div className="absolute inset-x-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-muted" />
        <div
          className="absolute top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-foreground/40"
          style={{ left: `${leftPct}%`, width: `${Math.max(rightPct - leftPct, 0)}%` }}
        />
        <input
          type="range"
          className={thumbClass}
          style={{ zIndex: fromOnTop ? 5 : 3 }}
          min={min}
          max={max}
          step={safeStep}
          value={lo}
          aria-label="Minimum"
          onChange={(e) => setFrom(Number(e.target.value))}
        />
        <input
          type="range"
          className={thumbClass}
          style={{ zIndex: fromOnTop ? 3 : 5 }}
          min={min}
          max={max}
          step={safeStep}
          value={hi}
          aria-label="Maximum"
          onChange={(e) => setTo(Number(e.target.value))}
        />
      </div>
      {showBounds ? (
        <div className="flex justify-between px-0.5 text-[11px] leading-none text-muted-foreground tabular-nums">
          <span>{formatOptionNumber(min, safeStep)}</span>
          <span>{formatOptionNumber(max, safeStep)}</span>
        </div>
      ) : null}
    </div>
  );
}

export function formatOptionRange(
  from: number,
  to: number,
  step?: number,
  unit?: string,
): string {
  const a = formatOptionNumber(Math.min(from, to), step);
  const b = formatOptionNumber(Math.max(from, to), step);
  const u = unit ? ` ${unit}` : "";
  return from === to || a === b ? `${a}${u}` : `${a}–${b}${u}`;
}
