export type StrategyKpiMeta = {
  title: string;
  slug: string;
  kind: string;
  label: string;
  unit?: string;
  /** Display order in tables; set in the README frontmatter. */
  order?: number;
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
