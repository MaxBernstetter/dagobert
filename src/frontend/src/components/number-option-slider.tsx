
import { formatOptionNumber } from "@/components/number-option-range-slider";
import { cn } from "@/lib/utils";

type NumberOptionSliderProps = {
  label?: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  unit?: string;
  description?: string;
  className?: string;
  /** When false, omit the value row (caller shows value elsewhere). */
  showValue?: boolean;
  onChange: (value: number) => void;
};

/** Single-thumb slider bounded to option min/max. */
export function NumberOptionSlider({
  label,
  value,
  min,
  max,
  step = 1,
  unit,
  description,
  className,
  showValue = true,
  onChange,
}: NumberOptionSliderProps) {
  const safeStep = step > 0 ? step : 1;
  const display = formatOptionNumber(value, safeStep);
  return (
    <label className={cn("grid gap-1", className)} title={description}>
      {showValue ? (
        <span className="flex items-baseline justify-between gap-2 text-sm leading-none">
          {label ? <span className="text-muted-foreground">{label}</span> : <span />}
          <span className="font-mono text-foreground tabular-nums">
            {display}
            {unit ? ` ${unit}` : ""}
          </span>
        </span>
      ) : null}
      <input
        type="range"
        className="h-4 w-full cursor-pointer appearance-none bg-transparent accent-foreground
          [&::-webkit-slider-runnable-track]:h-1.5 [&::-webkit-slider-runnable-track]:rounded-full
          [&::-webkit-slider-runnable-track]:bg-muted
          [&::-webkit-slider-thumb]:relative [&::-webkit-slider-thumb]:mt-[-5px]
          [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4
          [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full
          [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-background
          [&::-webkit-slider-thumb]:bg-foreground"
        min={min}
        max={max}
        step={safeStep}
        value={value}
        aria-valuemin={min}
        aria-valuemax={max}
        aria-valuenow={value}
        aria-label={label || "Value"}
        onChange={(e) => onChange(Number(e.target.value))}
      />
      <span className="flex justify-between text-[11px] leading-none text-muted-foreground tabular-nums">
        <span>{formatOptionNumber(min, safeStep)}</span>
        <span>{formatOptionNumber(max, safeStep)}</span>
      </span>
    </label>
  );
}
