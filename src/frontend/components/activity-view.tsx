"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { FilterableTable, type FilterableColumnDef } from "@/components/filterable-table";
import { StrategyPortfolioChart } from "@/components/strategy-portfolio-chart";
import { BacktestDetailSection } from "@/components/backtest-detail-section";
import { StrategyKpiTags } from "@/components/strategy-kpi-tags";
import type { ActivityRow, JournalRow, LiveState } from "@/lib/read";
import { portfolioSeriesForPlaybook, tradeMarkersFromActivity } from "@/lib/portfolio-series";
import type { StrategyGroup } from "@/lib/strategy-groups";
import { groupActivityByPlaybook } from "@/lib/strategy-groups";
import {
  executionKindHelp,
  rowExecutionHint,
} from "@/lib/execution-guidance";
import { STRATEGY_COLUMN_HELP } from "@/lib/strategy-help";
import { HelpTip } from "@/components/help-tip";
import { StatusBadge, rowVenueStatus } from "@/components/status-badge";
import { CandleDetail } from "@/components/candle-detail";
import { cn } from "@/lib/utils";
import { ChevronRight } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

const PAGE_SIZE = 20;
const BAR_SEC = 10_800;

function primarySymbol(rows: ActivityRow[]): string {
  const counts = new Map<string, number>();
  for (const row of rows) {
    counts.set(row.symbol, (counts.get(row.symbol) ?? 0) + 1);
  }
  const top = [...counts.entries()].sort((a, b) => b[1] - a[1])[0];
  return top?.[0] ?? "BTCUSDT";
}

function chartTimeRange(
  rows: ActivityRow[],
  points: { time: number }[],
  markers: { time: number }[]
): { fromSec: number; toSec: number } | null {
  const times: number[] = [];
  for (const row of rows) {
    if (row.candleClose > 0) times.push(Math.floor(row.candleClose / 1000));
  }
  for (const p of points) times.push(p.time);
  for (const m of markers) times.push(m.time);
  if (times.length === 0) return null;
  const min = Math.min(...times);
  const max = Math.max(...times);
  const pad = 30 * BAR_SEC;
  return { fromSec: min - pad, toSec: max + pad };
}

function formatCandleClose(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

function executionTooltip(row: ActivityRow, live: LiveState): string {
  const parts = [executionKindHelp(row.kind)];
  const rowHint = rowExecutionHint(row, live);
  if (rowHint) parts.push(rowHint);
  return parts.filter(Boolean).join(" ");
}

const activityColumns = (
  live: LiveState
): FilterableColumnDef<ActivityRow>[] => [
  {
    id: "candle",
    header: <HelpTip label="Candle" tip={STRATEGY_COLUMN_HELP.candle} />,
    filter: "text",
    filterValue: (row) => formatCandleClose(row.candleClose),
    cell: (row) => (
      <span className="whitespace-nowrap text-xs">{formatCandleClose(row.candleClose)}</span>
    ),
  },
  {
    id: "symbol",
    header: <HelpTip label="Symbol" tip={STRATEGY_COLUMN_HELP.symbol} />,
    filter: "select",
    filterValue: (row) => row.symbol,
    cell: (row) => row.symbol,
  },
  {
    id: "signal",
    header: <HelpTip label="Signal" tip={STRATEGY_COLUMN_HELP.signal} />,
    filter: "select",
    filterValue: (row) => row.signalDecision ?? "",
    cell: (row) => row.signalDecision ?? "—",
  },
  {
    id: "action",
    header: <HelpTip label="Action" tip={STRATEGY_COLUMN_HELP.action} />,
    filter: "select",
    filterValue: (row) => row.decision ?? "",
    cell: (row) => row.decision ?? "—",
  },
  {
    id: "size",
    header: (
      <HelpTip
        label="Size"
        tip="Sized amount from market-action (quote for buys, base for sells)."
      />
    ),
    filter: "text",
    filterValue: (row) => (row.size != null ? String(row.size) : ""),
    className: "tabular-nums",
    cell: (row) => (row.size != null ? String(row.size) : "—"),
  },
  {
    id: "execution",
    header: <HelpTip label="Execution" tip={STRATEGY_COLUMN_HELP.execution} />,
    filter: "select",
    filterValue: (row) => row.kind,
    cell: (row) => {
      const execTip = executionTooltip(row, live);
      return (
        <span title={execTip || undefined} className="cursor-help inline-flex">
          <StatusBadge kind={row.kind} />
        </span>
      );
    },
  },
  {
    id: "venue",
    header: <HelpTip label="Venue status" tip={STRATEGY_COLUMN_HELP.venue} />,
    filter: "text",
    filterValue: (row) => rowVenueStatus(row),
    className: "text-xs text-muted-foreground max-w-[10rem]",
    cell: (row) => {
      const venue = rowVenueStatus(row);
      return (
        <span className="line-clamp-2" title={venue !== "—" ? venue : undefined}>
          {venue}
        </span>
      );
    },
  },
];

function playbookWikiSlug(playbook: string): string {
  return playbook.replace(/\.ya?ml$/i, "");
}

type ReportApiScope = {
  backtestId: string;
  fromMs: number;
  toMs: number;
};

function StrategyRepeaterItem({
  group,
  live,
  journals,
  defaultOpen,
  selectedId,
  onSelect,
  reportApi,
}: {
  group: StrategyGroup;
  live: LiveState;
  journals: JournalRow[];
  defaultOpen: boolean;
  selectedId: string | null;
  onSelect: (id: string) => void;
  reportApi?: ReportApiScope;
}) {
  const columns = useMemo(() => activityColumns(live), [live]);
  const { points } = useMemo(
    () => portfolioSeriesForPlaybook(journals, group.playbook),
    [journals, group.playbook]
  );
  const markers = useMemo(() => tradeMarkersFromActivity(group.rows), [group.rows]);
  const wikiSlug = playbookWikiSlug(group.playbook);
  const symbol = useMemo(() => primarySymbol(group.rows), [group.rows]);
  const timeRange = useMemo(() => {
    if (reportApi) {
      const padSec = 12 * 3600;
      return {
        fromSec: Math.floor(reportApi.fromMs / 1000) - padSec,
        toSec: Math.ceil(reportApi.toMs / 1000) + padSec,
      };
    }
    return chartTimeRange(group.rows, points, markers);
  }, [reportApi, group.rows, points, markers]);
  const chartSeriesPath = useMemo(() => {
    if (!reportApi || !timeRange) return undefined;
    const q = new URLSearchParams({
      from: String(timeRange.fromSec),
      to: String(timeRange.toSec),
    });
    return `/api/backtests/${reportApi.backtestId}/chart-series?${q}`;
  }, [reportApi, timeRange]);

  const decisionsTable = (
    <FilterableTable
      rows={group.rows}
      columns={columns}
      rowKey={(row) => row.orderLinkId}
      emptyMessage="No decisions for this strategy yet."
      className="min-h-[16rem] max-h-[28rem]"
      pageSize={PAGE_SIZE}
      framed={false}
      onRowClick={(row) => onSelect(row.orderLinkId)}
      isRowSelected={(row) => row.orderLinkId === selectedId}
      rowClassName={(row) => {
        const isLatest = row.orderLinkId === group.rows[0]?.orderLinkId;
        const isSelected = row.orderLinkId === selectedId;
        return cn(
          isLatest && "bg-muted/60 border-l-4 border-l-primary font-medium",
          isSelected && "bg-muted",
        );
      }}
    />
  );

  if (reportApi) {
    return (
      <BacktestDetailSection title="Charts & decisions" defaultOpen={defaultOpen}>
        <div className="flex min-h-0 flex-col gap-4">
          {timeRange && (
            <StrategyPortfolioChart
              playbook={group.playbook}
              symbol={symbol}
              fromSec={timeRange.fromSec}
              toSec={timeRange.toSec}
              points={points}
              markers={markers}
              chartSeriesPath={chartSeriesPath}
            />
          )}
          {decisionsTable}
        </div>
      </BacktestDetailSection>
    );
  }

  return (
    <details
      open={defaultOpen}
      className="group rounded-xl border border-border/70 bg-muted/30 shadow-sm dark:bg-muted/15"
    >
      <summary
        className="flex cursor-pointer list-none items-start gap-2 px-4 py-3 [&::-webkit-details-marker]:hidden"
      >
        <ChevronRight
          className="mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90"
          aria-hidden
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <h3 className="text-base font-medium">{group.title}</h3>
            <StrategyKpiTags playbook={group.playbook} symbol={symbol} fromMs={0} toMs={0} />
            <Link
              href={`/wiki/playbooks/${wikiSlug}`}
              className="text-xs text-muted-foreground underline-offset-2 hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              Wiki
            </Link>
          </div>
          <p className="text-xs text-muted-foreground font-mono">{group.playbook}</p>
          <p className="text-sm text-muted-foreground mt-1">
            {group.decisionCount} decision{group.decisionCount === 1 ? "" : "s"} ·{" "}
            {group.fillCount} fill{group.fillCount === 1 ? "" : "s"}
            {group.awaitingExecution > 0
              ? ` · ${group.awaitingExecution} awaiting execution`
              : ""}
          </p>
        </div>
      </summary>
      <div className="flex min-h-0 flex-col gap-4 border-t border-border/50 px-4 pb-4 pt-4">
        {timeRange && (
          <Card className="bg-card shadow-md ring-1 ring-foreground/10">
            <CardHeader className="border-b border-border/60 pb-4">
              <CardTitle className="text-sm font-semibold">Portfolio &amp; signals</CardTitle>
              <CardDescription>
                Axis labels are UTC at 3h candle close. Buy/sell markers sit on the price chart (left) at the same
                time as the decisions table; portfolio equity is on the right.
              </CardDescription>
            </CardHeader>
            <CardContent className="pt-4">
              <StrategyPortfolioChart
                playbook={group.playbook}
                symbol={symbol}
                fromSec={timeRange.fromSec}
                toSec={timeRange.toSec}
                points={points}
                markers={markers}
                chartSeriesPath={chartSeriesPath}
              />
            </CardContent>
          </Card>
        )}
        <Card className="bg-card shadow-md ring-1 ring-foreground/10">
          <CardHeader className="border-b border-border/60 pb-4">
            <CardTitle className="text-sm font-semibold">Decisions</CardTitle>
            <CardDescription>
              Signals, sized actions, and execution status — newest first, 20 rows per page.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-4 pb-2">
            {decisionsTable}
          </CardContent>
        </Card>
      </div>
    </details>
  );
}

export function ActivityView({
  rows,
  journals,
  playbookTitles,
  live,
  className,
  reportApi,
  headingTitle = "Active Trading Strategies",
  headingDescription = "One panel per deployed playbook. Expand to see portfolio performance and recent decisions.",
}: {
  rows: ActivityRow[];
  journals: JournalRow[];
  playbookTitles: Record<string, string>;
  live: LiveState;
  className?: string;
  reportApi?: ReportApiScope;
  /** Ignored when `reportApi` is set (backtest detail embed). */
  headingTitle?: string;
  headingDescription?: string;
}) {
  const groups = useMemo(
    () => groupActivityByPlaybook(rows, playbookTitles),
    [rows, playbookTitles]
  );
  const [selectedId, setSelectedId] = useState<string | null>(rows[0]?.orderLinkId ?? null);
  const [detailOpen, setDetailOpen] = useState(false);

  const selected = useMemo(
    () => rows.find((r) => r.orderLinkId === selectedId) ?? null,
    [rows, selectedId]
  );

  function selectRow(id: string) {
    setSelectedId(id);
    setDetailOpen(true);
  }

  if (rows.length === 0) {
    return (
      <p className="text-sm text-muted-foreground text-center py-8">
        No strategy activity yet. After{" "}
        <code className="text-xs">task seed-demo-year</code>, wait until the per-bar pipeline starts
        (signal → execution); the page refreshes every 10s while open.
      </p>
    );
  }

  return (
    <>
      <div className={cn("flex min-h-0 flex-1 flex-col gap-4", className)}>
        {!reportApi && (
          <div className="shrink-0">
            <h2 className="text-lg font-semibold tracking-tight">{headingTitle}</h2>
            <p className="text-sm text-muted-foreground mt-0.5">{headingDescription}</p>
          </div>
        )}
        <div className={cn("flex min-h-0 flex-1 flex-col gap-3", !reportApi && "overflow-y-auto")}>
          {groups.map((group, index) => (
            <StrategyRepeaterItem
              key={group.playbook}
              group={group}
              live={live}
              journals={journals}
              defaultOpen={index === 0}
              selectedId={selectedId}
              onSelect={selectRow}
              reportApi={reportApi}
            />
          ))}
        </div>
      </div>

      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent
          side="right"
          className="flex h-full w-full flex-col gap-0 p-0 sm:max-w-xl lg:max-w-2xl"
        >
          <SheetHeader className="border-b px-4 py-3">
            <SheetTitle>Strategy details</SheetTitle>
          </SheetHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
            <CandleDetail row={selected} live={live} />
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}
