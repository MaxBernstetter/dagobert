export function tradeCountFromFills(fills: number): number {
  return Math.floor(fills / 2);
}

export type EquityPoint = { closeMs: number; value: number };

export type PeriodReturnPoint = { period: string; return: number };

export type OutcomeDistribution = {
  average: number;
  p10: number;
  p25: number;
  p50: number;
  p75: number;
  p90: number;
  p95: number;
};

export type OutcomeSide = {
  absolute?: OutcomeDistribution;
  percentage?: OutcomeDistribution;
};

export type BacktestRunStatistics = {
  durationMonths: number;
  durationYears: number;
  monthsSampled: number;
  yearsSampled: number;
  monthlyReturnAvg: number | null;
  monthlyReturnMed: number | null;
  yearlyReturnAvg: number | null;
  yearlyReturnMed: number | null;
  tradesPerMonthAvg: number | null;
  tradesPerMonthMed: number | null;
  winRate?: number | null;
  monthlyWinRateAvg?: number | null;
  monthlyWinRateMed?: number | null;
  yearlyWinRateAvg?: number | null;
  yearlyWinRateMed?: number | null;
  gain?: OutcomeSide | null;
  loss?: OutcomeSide | null;
  /** Optional series for charts (from report.json / KPI API). */
  monthlyReturns?: PeriodReturnPoint[];
  yearlyReturns?: PeriodReturnPoint[];
};

export function runDurationYears(fromMs: number, toMs: number): number {
  const span = Math.max(0, toMs - fromMs);
  return span / (365.25 * 24 * 60 * 60 * 1000);
}

export function runDurationMonths(fromMs: number, toMs: number): number {
  return runDurationYears(fromMs, toMs) * 12;
}

export function avgMed(values: number[]): { avg: number | null; med: number | null } {
  const nums = values.filter((v) => Number.isFinite(v));
  if (nums.length === 0) return { avg: null, med: null };
  const sum = nums.reduce((a, b) => a + b, 0);
  const sorted = [...nums].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  const med =
    sorted.length % 2 === 0
      ? (sorted[mid - 1] + sorted[mid]) / 2
      : sorted[mid];
  return { avg: sum / nums.length, med };
}

function utcMonthKey(ms: number): string {
  const d = new Date(ms);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

function utcYearKey(ms: number): string {
  return String(new Date(ms).getUTCFullYear());
}

/** Period return from first to last equity point in each bucket. */
function periodReturnsFromCurve(
  curve: EquityPoint[],
  bucketKey: (ms: number) => string,
): number[] {
  if (curve.length < 2) return [];
  const buckets = new Map<string, EquityPoint[]>();
  for (const p of curve) {
    const key = bucketKey(p.closeMs);
    const list = buckets.get(key) ?? [];
    list.push(p);
    buckets.set(key, list);
  }
  const returns: number[] = [];
  for (const points of buckets.values()) {
    if (points.length < 1) continue;
    points.sort((a, b) => a.closeMs - b.closeMs);
    const start = points[0].value;
    const end = points[points.length - 1].value;
    if (start > 0 && end > 0) {
      returns.push(end / start - 1);
    }
  }
  return returns;
}

/** Completed trades ≈ sell fills per calendar month. */
function monthlyTradeCounts(fills: { candleCloseMs: number; side: string }[]): number[] {
  const buckets = new Map<string, number>();
  for (const f of fills) {
    if (f.side !== "sell") continue;
    const key = utcMonthKey(f.candleCloseMs);
    buckets.set(key, (buckets.get(key) ?? 0) + 1);
  }
  return [...buckets.values()];
}

export function computeRunStatistics(
  portfolio: EquityPoint[],
  fills: { candleCloseMs: number; side: string }[],
  fromMs: number,
  toMs: number,
): BacktestRunStatistics {
  const durationYears = runDurationYears(fromMs, toMs);
  const durationMonths = durationYears * 12;

  const monthlyReturns = periodReturnsFromCurve(portfolio, utcMonthKey);
  const yearlyReturns = periodReturnsFromCurve(portfolio, utcYearKey);
  const monthlyRet = avgMed(monthlyReturns);
  const yearlyRet = avgMed(yearlyReturns);

  const tradeMonths = monthlyTradeCounts(fills);
  const tradeRates = avgMed(tradeMonths);

  return {
    durationMonths,
    durationYears,
    monthsSampled: monthlyReturns.length,
    yearsSampled: yearlyReturns.length,
    monthlyReturnAvg: monthlyRet.avg,
    monthlyReturnMed: monthlyRet.med,
    yearlyReturnAvg: yearlyRet.avg,
    yearlyReturnMed: yearlyRet.med,
    tradesPerMonthAvg: tradeRates.avg,
    tradesPerMonthMed: tradeRates.med,
  };
}

export function monthlyEventCounts(timestampsMs: number[]): number[] {
  const buckets = new Map<string, number>();
  for (const ms of timestampsMs) {
    if (!Number.isFinite(ms) || ms <= 0) continue;
    const key = utcMonthKey(ms);
    buckets.set(key, (buckets.get(key) ?? 0) + 1);
  }
  return [...buckets.values()];
}

export function decisionsPerMonthStats(timestampsMs: number[]): {
  avg: number | null;
  med: number | null;
} {
  return avgMed(monthlyEventCounts(timestampsMs));
}
