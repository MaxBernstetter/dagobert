/** Default KPI defs (mirrors Go StrategyKPIRegistry); used when markdown catalog is unavailable. */
export function defaultStrategyKpiDefs(): StrategyKpiMeta[] {
  return [
    {
      slug: "cagr",
      title: "CAGR (net)",
      label: "CAGR",
      kind: "strategy-kpi",
      unit: "ratio",
      format: "percent",
    },
    {
      slug: "max-drawdown",
      title: "Maximum drawdown",
      label: "Max DD",
      kind: "strategy-kpi",
      unit: "ratio",
      format: "percent",
    },
    {
      slug: "sharpe",
      title: "Sharpe ratio",
      label: "Sharpe",
      kind: "strategy-kpi",
      unit: "ratio",
      format: "decimal",
    },
    {
      slug: "active-return",
      title: "Active return vs benchmark",
      label: "vs B&H",
      kind: "strategy-kpi",
      unit: "ratio",
      format: "percent-pts",
    },
    {
      slug: "sortino",
      title: "Sortino ratio",
      label: "Sortino",
      kind: "strategy-kpi",
      unit: "ratio",
      format: "decimal",
    },
    {
      slug: "win-rate",
      title: "Win rate",
      label: "Win rate",
      kind: "strategy-kpi",
      unit: "ratio",
      format: "percent",
    },
  ];
}

export type StrategyKpiMeta = {
  title: string;
  slug: string;
  kind: string;
  label: string;
  unit?: string;
  format: "percent" | "decimal" | "percent-pts" | string;
};

export type StrategyKpiResult = {
  def: StrategyKpiMeta;
  value: number;
  extra?: Record<string, number>;
};

export type StrategyKpiComparison = {
  startCash: number;
  strategyReturn: number;
  buyHoldReturn: number;
};

import type { BacktestRunStatistics } from "@/lib/backtest-period-stats";

export type { BacktestRunStatistics };

export type StrategyKpiSet = {
  playbook: string;
  symbol: string;
  fromMs: number;
  toMs: number;
  results: StrategyKpiResult[];
  comparison?: StrategyKpiComparison;
  statistics?: BacktestRunStatistics;
};

export function formatStrategyKpiValue(format: string, value: number): string {
  if (!Number.isFinite(value)) {
    return "—";
  }
  switch (format) {
    case "percent":
      return `${(value * 100).toFixed(1)}%`;
    case "percent-pts": {
      const pts = value * 100;
      const sign = pts > 0 ? "+" : "";
      return `${sign}${pts.toFixed(1)}pp`;
    }
    case "decimal":
      return value.toFixed(2);
    default:
      return String(value);
  }
}
