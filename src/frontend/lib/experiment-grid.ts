/** Max valid combinations allowed for one GRID create. */
export const MAX_GRID_COMBOS = 20_000;

/** Synthetic GRID axis key for market symbol (not a strategy option). */
export const GRID_SYMBOL_KEY = "symbol";

/** Default GRID markets when the provider list is unavailable. */
export const EXPERIMENT_SYMBOLS = [
  { value: "BTCUSDT", label: "BTCUSDT" },
  { value: "ETHUSDT", label: "ETHUSDT" },
] as const;

/** Prefer common pipeline symbols that exist in `options`; else first two. */
export function defaultGridSymbols(
  options: Array<{ value: string; label: string }>,
): string[] {
  if (options.length === 0) return EXPERIMENT_SYMBOLS.map((s) => s.value);
  const preferred = EXPERIMENT_SYMBOLS.map((s) => s.value).filter((v) =>
    options.some((o) => o.value === v),
  );
  if (preferred.length > 0) return preferred;
  return options.slice(0, 2).map((o) => o.value);
}

export type GridAxis = {
  key: string;
  values: Array<number | string>;
};

export type GridAxisDraft =
  | {
      key: string;
      kind: "enum";
      /** Selected enum values (multi-select). */
      selected: Array<number | string>;
    }
  | {
      key: string;
      kind: "number";
      from: number;
      to: number;
      tick: number;
    };

/** Inclusive arithmetic sequence from→to with positive tick. Empty if invalid. */
export function numberAxisValues(from: number, to: number, tick: number): number[] {
  if (!Number.isFinite(from) || !Number.isFinite(to) || !Number.isFinite(tick) || tick <= 0) {
    return [];
  }
  if (to < from) return [];
  const out: number[] = [];
  // Avoid float drift for common integer ticks
  const steps = Math.floor((to - from) / tick + 1e-9);
  for (let i = 0; i <= steps; i++) {
    const v = from + i * tick;
    if (v > to + 1e-9) break;
    out.push(Number(v.toPrecision(12)));
  }
  return out;
}

export function axisFromDraft(draft: GridAxisDraft): GridAxis {
  if (draft.kind === "enum") {
    return { key: draft.key, values: [...draft.selected] };
  }
  return {
    key: draft.key,
    values: numberAxisValues(draft.from, draft.to, draft.tick),
  };
}

/** Cartesian product of axis values → config objects. */
export function expandGrid(axes: GridAxis[]): Array<Record<string, number | string>> {
  if (axes.length === 0) return [{}];
  let combos: Array<Record<string, number | string>> = [{}];
  for (const axis of axes) {
    if (axis.values.length === 0) return [];
    const next: Array<Record<string, number | string>> = [];
    for (const base of combos) {
      for (const v of axis.values) {
        next.push({ ...base, [axis.key]: v });
      }
    }
    combos = next;
  }
  return combos;
}

const ALLOWED_SMA = new Set([3, 6, 12, 24, 48]);

/** Mirror catalog ma-cross validation (drop invalid without throwing). Ignores symbol. */
export function isValidMaCrossConfig(cfg: Record<string, number | string>): boolean {
  const fastHr = Number(cfg.fastHr);
  const slowHr = Number(cfg.slowHr);
  if (!ALLOWED_SMA.has(fastHr) || !ALLOWED_SMA.has(slowHr)) return false;
  if (fastHr >= slowHr) return false;
  return true;
}

/** Mirror catalog signal-consensus threshold validation. */
export function isValidSignalConsensusConfig(cfg: Record<string, number | string>): boolean {
  const rsiBull = Number(cfg.rsiBull);
  const rsiBear = Number(cfg.rsiBear);
  const volRatioBull = Number(cfg.volRatioBull);
  const volRatioBear = Number(cfg.volRatioBear);
  const atrStopMult = Number(cfg.atrStopMult);
  const minFlat = Number(cfg.minBarsFlatAfterSell);
  const minLong = Number(cfg.minBarsLongBeforeSell);
  if (!(rsiBull > rsiBear)) return false;
  if (!(volRatioBull > volRatioBear) || volRatioBull <= 0 || volRatioBear <= 0) return false;
  if (!(atrStopMult > 0)) return false;
  if (!Number.isFinite(minFlat) || minFlat < 0 || !Number.isFinite(minLong) || minLong < 0) {
    return false;
  }
  return true;
}

export function filterValidConfigs(
  configs: Array<Record<string, number | string>>,
  playbook: string,
): Array<Record<string, number | string>> {
  const slug = playbook.replace(/\.ya?ml$/i, "").trim();
  if (slug === "ma-cross") {
    return configs.filter((c) => {
      const sym = c[GRID_SYMBOL_KEY];
      if (sym != null && String(sym).trim() === "") return false;
      return isValidMaCrossConfig(c);
    });
  }
  if (slug === "signal-consensus") {
    return configs.filter((c) => {
      const sym = c[GRID_SYMBOL_KEY];
      if (sym != null && String(sym).trim() === "") return false;
      return isValidSignalConsensusConfig(c);
    });
  }
  return configs;
}

/** Split a GRID combo into instance symbol + strategy config fields. */
export function splitGridCombo(cfg: Record<string, number | string>): {
  symbol: string | undefined;
  strategyConfig: Record<string, number | string>;
} {
  const strategyConfig: Record<string, number | string> = {};
  let symbol: string | undefined;
  for (const [k, v] of Object.entries(cfg)) {
    if (k === GRID_SYMBOL_KEY) {
      symbol = String(v);
      continue;
    }
    strategyConfig[k] = v;
  }
  return { symbol, strategyConfig };
}

export type GridPreview = {
  rawCount: number;
  valid: Array<Record<string, number | string>>;
  validCount: number;
  ok: boolean;
  error: string | null;
};

export function previewGrid(
  drafts: GridAxisDraft[],
  playbook: string,
): GridPreview {
  const axes = drafts.map(axisFromDraft);
  if (axes.some((a) => a.values.length === 0)) {
    return {
      rawCount: 0,
      valid: [],
      validCount: 0,
      ok: false,
      error: "Each option needs at least one value (select enum values or set a valid from/to/tick)",
    };
  }
  const raw = expandGrid(axes);
  const valid = filterValidConfigs(raw, playbook);
  if (valid.length === 0) {
    return {
      rawCount: raw.length,
      valid: [],
      validCount: 0,
      ok: false,
      error:
        "No valid combinations (check constraints, e.g. fast < slow or rsiBull > rsiBear)",
    };
  }
  if (valid.length > MAX_GRID_COMBOS) {
    return {
      rawCount: raw.length,
      valid,
      validCount: valid.length,
      ok: false,
      error: `Too many valid combinations (${valid.length}); max is ${MAX_GRID_COMBOS}`,
    };
  }
  return {
    rawCount: raw.length,
    valid,
    validCount: valid.length,
    ok: true,
    error: null,
  };
}
