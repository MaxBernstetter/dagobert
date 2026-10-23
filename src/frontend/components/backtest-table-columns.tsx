"use client";

import type { FilterableColumnDef } from "@/components/filterable-table";
import { BacktestStatusBadge } from "@/components/backtest-status-badge";
import {
  BACKTEST_TABLE_KPI_DEFS,
  BACKTEST_TABLE_STAT_COLUMNS,
  formatKpiCell,
  formatStatPercentCell,
  formatStatRateCell,
  kpiResultValue,
} from "@/lib/backtest-list-kpi";
import {
  backtestFromMsToDateInput,
  backtestStatusLabel,
  backtestToMsToDateInput,
  type BacktestRecord,
} from "@/lib/backtest-shared";
import type { StrategyKpiSet } from "@/lib/strategy-kpi-shared";

function kpiFor(
  kpiById: Record<string, StrategyKpiSet | null>,
  row: BacktestRecord,
): StrategyKpiSet | null | undefined {
  if (row.status !== "completed") return undefined;
  return kpiById[row.id];
}

function kpiLoading(loadingIds: Set<string>, row: BacktestRecord): boolean {
  return row.status === "completed" && loadingIds.has(row.id);
}

export function buildBacktestTableColumns(
  kpiById: Record<string, StrategyKpiSet | null>,
  loadingIds: Set<string>,
): FilterableColumnDef<BacktestRecord>[] {
  const cols: FilterableColumnDef<BacktestRecord>[] = [
    {
      id: "name",
      header: "Name",
      filter: "text",
      filterValue: (r) => r.name,
      sortValue: (r) => r.name.toLowerCase(),
      cell: (r) => <span className="font-medium">{r.name}</span>,
    },
    {
      id: "symbol",
      header: "Symbol",
      filter: "text",
      filterValue: (r) => r.settings.symbol,
      sortValue: (r) => r.settings.symbol,
      className: "font-mono text-xs",
      cell: (r) => r.settings.symbol,
    },
    {
      id: "from",
      header: "From",
      filter: "text",
      filterValue: (r) => backtestFromMsToDateInput(r.settings.fromMs),
      sortValue: (r) => r.settings.fromMs,
      className: "text-xs whitespace-nowrap tabular-nums",
      cell: (r) => backtestFromMsToDateInput(r.settings.fromMs),
    },
    {
      id: "to",
      header: "To",
      filter: "text",
      filterValue: (r) => backtestToMsToDateInput(r.settings.toMs),
      sortValue: (r) => r.settings.toMs,
      className: "text-xs whitespace-nowrap tabular-nums",
      cell: (r) => backtestToMsToDateInput(r.settings.toMs),
    },
    {
      id: "status",
      header: "Status",
      filter: "text",
      filterValue: (r) => backtestStatusLabel(r),
      sortValue: (r) => r.status,
      className: "text-xs",
      cell: (r) => (
        <BacktestStatusBadge
          status={r.status}
          progressDone={r.progressDone}
          progressTotal={r.progressTotal}
          progressStage={r.progressStage}
          progressStages={r.progressStages}
          progressStageLabel={r.progressStageLabel}
        />
      ),
    },
    {
      id: "playbook",
      header: "Playbook",
      filter: "text",
      filterValue: (r) => r.settings.playbook,
      sortValue: (r) => r.settings.playbook,
      className: "font-mono text-xs",
      cell: (r) => r.settings.playbook,
    },
  ];

  for (const def of BACKTEST_TABLE_KPI_DEFS) {
    cols.push({
      id: `kpi-${def.slug}`,
      header: def.label,
      filter: "text",
      filterValue: (r) =>
        formatKpiCell(kpiFor(kpiById, r), def.slug, def.format, kpiLoading(loadingIds, r)),
      sortValue: (r) => kpiResultValue(kpiFor(kpiById, r), def.slug),
      className: "tabular-nums text-xs",
      cell: (r) =>
        formatKpiCell(kpiFor(kpiById, r), def.slug, def.format, kpiLoading(loadingIds, r)),
    });
  }

  for (const col of BACKTEST_TABLE_STAT_COLUMNS) {
    cols.push({
      id: col.id,
      header: col.header,
      filter: "text",
      filterValue: (r) => {
        const set = kpiFor(kpiById, r);
        const loading = kpiLoading(loadingIds, r);
        return col.format === "percent"
          ? formatStatPercentCell(set, col.pick, loading)
          : formatStatRateCell(set, col.pick, loading);
      },
      sortValue: (r) => {
        const set = kpiFor(kpiById, r);
        const v = set?.statistics ? col.pick(set.statistics) : null;
        return v != null && Number.isFinite(v) ? v : null;
      },
      className: "tabular-nums text-xs",
      cell: (r) => {
        const set = kpiFor(kpiById, r);
        const loading = kpiLoading(loadingIds, r);
        return col.format === "percent"
          ? formatStatPercentCell(set, col.pick, loading)
          : formatStatRateCell(set, col.pick, loading);
      },
    });
  }

  return cols;
}
