import type { BacktestRunStatistics } from "@/lib/backtest-period-stats";
import { strategyKpiDefs } from "@/wikiCatalog";
import { formatStrategyKpiValue, type StrategyKpiSet } from "@/lib/strategy-kpi-shared";

export function kpiResultValue(set: StrategyKpiSet | null | undefined, slug: string): number | null {
  if (!set) return null;
  const row = set.results.find((r) => r.def.slug === slug);
  if (row == null || !Number.isFinite(row.value)) return null;
  return row.value;
}

export function formatKpiCell(
  set: StrategyKpiSet | null | undefined,
  slug: string,
  format: string,
  loading: boolean,
): string {
  if (loading) return "…";
  const v = kpiResultValue(set, slug);
  if (v == null) return "—";
  return formatStrategyKpiValue(format, v);
}

function statNum(
  stats: BacktestRunStatistics | null | undefined,
  pick: (s: BacktestRunStatistics) => number | null,
): number | null {
  if (!stats) return null;
  const v = pick(stats);
  return v != null && Number.isFinite(v) ? v : null;
}

export function formatStatPercentCell(
  set: StrategyKpiSet | null | undefined,
  pick: (s: BacktestRunStatistics) => number | null,
  loading: boolean,
): string {
  if (loading) return "…";
  const v = statNum(set?.statistics, pick);
  if (v == null) return "—";
  return formatStrategyKpiValue("percent", v);
}

export function formatStatRateCell(
  set: StrategyKpiSet | null | undefined,
  pick: (s: BacktestRunStatistics) => number | null,
  loading: boolean,
): string {
  if (loading) return "…";
  const v = statNum(set?.statistics, pick);
  if (v == null) return "—";
  return v.toFixed(1);
}

/** KPIs suitable for cross-run table comparison (time-normalized or risk). */
export const BACKTEST_TABLE_KPI_DEFS = strategyKpiDefs().filter(
  (d) => d.slug !== "active-return",
);

export const BACKTEST_TABLE_STAT_COLUMNS: {
  id: string;
  header: string;
  format: "percent" | "rate";
  pick: (s: BacktestRunStatistics) => number | null;
}[] = [
  { id: "mo-ret-avg", header: "Mo ret μ", format: "percent", pick: (s) => s.monthlyReturnAvg },
  { id: "mo-ret-med", header: "Mo ret med", format: "percent", pick: (s) => s.monthlyReturnMed },
  { id: "yr-ret-avg", header: "Yr ret μ", format: "percent", pick: (s) => s.yearlyReturnAvg },
  { id: "yr-ret-med", header: "Yr ret med", format: "percent", pick: (s) => s.yearlyReturnMed },
  { id: "trades-mo-avg", header: "Trades/mo μ", format: "rate", pick: (s) => s.tradesPerMonthAvg },
  { id: "trades-mo-med", header: "Trades/mo med", format: "rate", pick: (s) => s.tradesPerMonthMed },
  { id: "mo-wr-avg", header: "Mo win μ", format: "percent", pick: (s) => s.monthlyWinRateAvg ?? null },
  { id: "mo-wr-med", header: "Mo win med", format: "percent", pick: (s) => s.monthlyWinRateMed ?? null },
  { id: "yr-wr-avg", header: "Yr win μ", format: "percent", pick: (s) => s.yearlyWinRateAvg ?? null },
  { id: "yr-wr-med", header: "Yr win med", format: "percent", pick: (s) => s.yearlyWinRateMed ?? null },
];
