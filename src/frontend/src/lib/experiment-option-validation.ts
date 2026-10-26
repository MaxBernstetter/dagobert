/** Schema and clamp helpers for experiment strategy number options. */

export type NumberOptionBounds = {
  min: number;
  max: number;
};

/** Schema: every number option must declare finite min < max. */
export function isValidNumberOptionBounds(bounds: NumberOptionBounds): boolean {
  const { min, max } = bounds;
  return Number.isFinite(min) && Number.isFinite(max) && min < max;
}

/** Clamp a value into [min, max] (slider-safe). */
export function clampNumber(value: number, bounds: NumberOptionBounds): number {
  if (!Number.isFinite(value)) return bounds.min;
  if (value < bounds.min) return bounds.min;
  if (value > bounds.max) return bounds.max;
  return value;
}

/**
 * Dual-thumb GRID range: keep Minimum ≤ Maximum while both stay in bounds.
 * Moving one past the other pulls the other along.
 */
export function clampGridRange(
  from: number,
  to: number,
  bounds: NumberOptionBounds,
  moved: "from" | "to",
): { from: number; to: number } {
  let nextFrom = clampNumber(from, bounds);
  let nextTo = clampNumber(to, bounds);
  if (moved === "from" && nextFrom > nextTo) nextTo = nextFrom;
  if (moved === "to" && nextTo < nextFrom) nextFrom = nextTo;
  return { from: nextFrom, to: nextTo };
}
