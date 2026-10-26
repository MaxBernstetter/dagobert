
import type { FilterableColumnDef } from "@/components/filterable-table";
import { CatalogStatusBadge } from "@/components/backtest-status-badge";
import {
  BACKTEST_TABLE_KPI_DEFS,
  BACKTEST_TABLE_STAT_COLUMNS,
} from "@/lib/backtest-list-kpi";
import { formatStrategyKpiValue } from "@/lib/strategy-kpi-shared";
import type { ExperimentInstance, ExperimentInstanceMetrics } from "@/lib/experiment-shared";
import {
  returnTone,
  strategyKpiTone,
  toneTextClassName,
} from "@/lib/strategy-kpi-tone";
import { cn } from "@/lib/utils";

function instanceConfigLabel(inst: ExperimentInstance): string {
  const cfg = inst.strategyConfig as Record<string, unknown> | null | undefined;
  if (!cfg || typeof cfg !== "object") return "—";
  if (typeof cfg.fastHr === "number" && typeof cfg.slowHr === "number") {
    return `${cfg.fastHr}h / ${cfg.slowHr}h`;
  }
  if (typeof cfg.rsiBull === "number" && typeof cfg.rsiBear === "number") {
    const atr = typeof cfg.atrStopMult === "number" ? cfg.atrStopMult : "?";
    return `rsi ${cfg.rsiBull}/${cfg.rsiBear} atr×${atr}`;
  }
  return "—";
}

function metricNum(
  m: ExperimentInstanceMetrics | undefined,
  key: keyof ExperimentInstanceMetrics,
): number | null {
  if (!m) return null;
  const v = m[key];
  return v != null && Number.isFinite(v) ? v : null;
}

function MetricCell({
  value,
  format,
  tone,
}: {
  value: number | null;
  format: "percent" | "decimal" | "rate";
  tone?: (v: number) => string;
}) {
  if (value == null) return <span className="text-muted-foreground">—</span>;
  const text =
    format === "rate"
      ? value.toFixed(1)
      : formatStrategyKpiValue(format === "percent" ? "percent" : "decimal", value);
  return <span className={cn("tabular-nums", tone?.(value))}>{text}</span>;
}

const KPI_METRIC_KEY: Record<string, keyof ExperimentInstanceMetrics> = {
  cagr: "cagr",
  "max-drawdown": "maxDrawdown",
  sharpe: "sharpe",
  sortino: "sortino",
  "win-rate": "winRate",
};

/** Column id → sort key the Experiment API accepts. Columns not listed here are not sortable on the server. */
export const SERVER_SORT_KEY: Record<string, string> = {
  label: "label",
  config: "config",
  symbol: "symbol",
  status: "status",
  "kpi-cagr": "cagr",
  "kpi-max-drawdown": "maxDrawdown",
  "kpi-sharpe": "sharpe",
  "kpi-sortino": "sortino",
  "kpi-win-rate": "winRate",
  "mo-ret-avg": "monthlyReturnAvg",
  "mo-ret-med": "monthlyReturnMed",
  "yr-ret-avg": "yearlyReturnAvg",
  "yr-ret-med": "yearlyReturnMed",
  "trades-mo-avg": "tradesPerMonthAvg",
  "trades-mo-med": "tradesPerMonthMed",
  "mo-wr-avg": "monthlyWinRateAvg",
  "mo-wr-med": "monthlyWinRateMed",
  "yr-wr-avg": "yearlyWinRateAvg",
  "yr-wr-med": "yearlyWinRateMed",
};

/** Columns shown until the user picks others: drawdown, annual and monthly return. */
export const DEFAULT_VISIBLE_INSTANCE_COLUMNS = [
  "label",
  "symbol",
  "config",
  "status",
  "kpi-max-drawdown",
  "kpi-cagr",
  "mo-ret-avg",
];

const STAT_METRIC_KEY: Record<string, keyof ExperimentInstanceMetrics> = {
  "mo-ret-avg": "monthlyReturnAvg",
  "mo-ret-med": "monthlyReturnMed",
  "yr-ret-avg": "yearlyReturnAvg",
  "yr-ret-med": "yearlyReturnMed",
  "trades-mo-avg": "tradesPerMonthAvg",
  "trades-mo-med": "tradesPerMonthMed",
  "mo-wr-avg": "monthlyWinRateAvg",
  "mo-wr-med": "monthlyWinRateMed",
  "yr-wr-avg": "yearlyWinRateAvg",
  "yr-wr-med": "yearlyWinRateMed",
};

function kpiTone(slug: string, v: number): string {
  return toneTextClassName(strategyKpiTone(slug, v));
}

/** Columns for server-sorted instance pages — metrics come from the row payload. */
export function buildExperimentInstanceTableColumns(
  defaultSymbol: string,
): FilterableColumnDef<ExperimentInstance>[] {
  const cols: FilterableColumnDef<ExperimentInstance>[] = [
    {
      id: "label",
      header: "Label",
      filter: "text",
      filterValue: (r) => r.label,
      sortValue: (r) => r.label.toLowerCase(),
      cell: (r) => <span className="font-medium">{r.label}</span>,
    },
    {
      id: "symbol",
      header: "Symbol",
      filter: "text",
      filterValue: (r) => r.symbol || defaultSymbol,
      sortValue: (r) => r.symbol || defaultSymbol,
      className: "font-mono text-xs",
      cell: (r) => r.symbol || defaultSymbol,
    },
    {
      id: "config",
      header: "Config",
      filter: "text",
      filterValue: (r) => instanceConfigLabel(r),
      sortValue: (r) => instanceConfigLabel(r),
      className: "font-mono text-xs",
      cell: (r) => instanceConfigLabel(r),
    },
    {
      id: "status",
      header: "Status",
      filter: "text",
      filterValue: (r) => r.status,
      sortValue: (r) => r.status,
      className: "text-xs",
      cell: (r) => <CatalogStatusBadge status={r.status} />,
    },
  ];

  for (const def of BACKTEST_TABLE_KPI_DEFS) {
    const key = KPI_METRIC_KEY[def.slug];
    if (!key) continue;
    cols.push({
      id: `kpi-${def.slug}`,
      header: def.label,
      filter: "text",
      filterValue: (r) => {
        const v = metricNum(r.metrics, key);
        return v == null ? "—" : formatStrategyKpiValue(def.format, v);
      },
      sortValue: (r) => metricNum(r.metrics, key),
      className: "tabular-nums text-xs",
      cell: (r) => (
        <MetricCell
          value={metricNum(r.metrics, key)}
          format={def.format === "percent" ? "percent" : "decimal"}
          tone={(v) => kpiTone(def.slug, v)}
        />
      ),
    });
  }

  for (const col of BACKTEST_TABLE_STAT_COLUMNS) {
    const key = STAT_METRIC_KEY[col.id];
    if (!key) continue;
    cols.push({
      id: col.id,
      header: col.header,
      filter: "text",
      filterValue: (r) => {
        const v = metricNum(r.metrics, key);
        if (v == null) return "—";
        return col.format === "percent"
          ? formatStrategyKpiValue("percent", v)
          : v.toFixed(1);
      },
      sortValue: (r) => metricNum(r.metrics, key),
      className: "tabular-nums text-xs",
      cell: (r) => (
        <MetricCell
          value={metricNum(r.metrics, key)}
          format={col.format === "percent" ? "percent" : "rate"}
          tone={
            col.format === "percent"
              ? (v) =>
                  toneTextClassName(
                    returnTone(v * (col.id.startsWith("yr") ? 1 : 12)),
                  )
              : undefined
          }
        />
      ),
    });
  }

  return cols;
}
