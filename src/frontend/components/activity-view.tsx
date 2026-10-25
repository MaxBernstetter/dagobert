"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { flushSync } from "react-dom";
import Link from "next/link";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { FilterableTable, type FilterableColumnDef } from "@/components/filterable-table";
import { StrategyCharts } from "@/components/strategy-charts";
import { BacktestDetailSection } from "@/components/backtest-detail-section";
import { Badge } from "@/components/ui/badge";
import { StrategyKpiTags } from "@/components/strategy-kpi-tags";
import type { ActivityRow, JournalRow, LiveState } from "@/types/read";
import { portfolioSeriesForPlaybook, tradeMarkersFromActivity } from "@/lib/portfolio-series";
import type { StrategyGroup } from "@/lib/strategy-groups";
import { groupActivityByPlaybook } from "@/lib/strategy-groups";
import {
  executionKindHelp,
  rowExecutionHint,
} from "@/lib/execution-guidance";
import { STRATEGY_COLUMN_HELP } from "@/lib/strategy-help";
import { HelpTip } from "@/components/help-tip";
import { useViewportHeight } from "@/hooks/use-viewport-height";
import { StatusBadge, rowVenueStatus, venueStatusAddsDetail } from "@/components/status-badge";
import { CandleDetail } from "@/components/candle-detail";
import { cn } from "@/lib/utils";
import { ChevronRight, Loader2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

const PAGE_SIZE = 20;
/** Instance report: performance stays in the left track from the first paint. */
const reportColumnsClass =
  "flex min-h-0 flex-1 flex-col gap-3 xl:grid xl:h-full xl:min-h-0 xl:grid-cols-[minmax(40rem,48rem)_minmax(0,1fr)] xl:grid-rows-[minmax(0,1fr)] xl:gap-4";
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
    sortValue: (row) => row.candleClose,
    cell: (row) => (
      <span className="whitespace-nowrap text-xs">{formatCandleClose(row.candleClose)}</span>
    ),
  },
  {
    id: "symbol",
    header: <HelpTip label="Symbol" tip={STRATEGY_COLUMN_HELP.symbol} />,
    filter: "select",
    filterValue: (row) => row.symbol,
    sortValue: (row) => row.symbol,
    cell: (row) => row.symbol,
  },
  {
    id: "signal",
    header: <HelpTip label="Signal" tip={STRATEGY_COLUMN_HELP.signal} />,
    filter: "select",
    filterValue: (row) => row.signalDecision ?? "",
    sortValue: (row) => row.signalDecision ?? "",
    cell: (row) => row.signalDecision ?? "—",
  },
  {
    id: "action",
    header: <HelpTip label="Action" tip={STRATEGY_COLUMN_HELP.action} />,
    filter: "select",
    filterValue: (row) => row.decision ?? "",
    sortValue: (row) => row.decision ?? "",
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
    sortValue: (row) => row.size,
    className: "tabular-nums",
    cell: (row) => (row.size != null ? String(row.size) : "—"),
  },
  {
    id: "execution",
    header: <HelpTip label="Execution" tip={STRATEGY_COLUMN_HELP.execution} />,
    filter: "select",
    filterValue: (row) => row.kind,
    sortValue: (row) => row.kind,
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
    sortValue: (row) => rowVenueStatus(row),
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

/** Instance runs share one asset and often omit signal, size, and a distinct venue status. */
function columnsForRows(live: LiveState, rows: ActivityRow[], compact: boolean) {
  const cols = activityColumns(live);
  if (!compact || rows.length === 0) return cols;
  const drop = new Set<string>();
  const symbol = rows[0]?.symbol;
  if (symbol && rows.every((row) => row.symbol === symbol)) drop.add("symbol");
  if (rows.every((row) => !row.signalDecision)) drop.add("signal");
  if (rows.every((row) => row.size == null)) drop.add("size");
  if (rows.every((row) => !venueStatusAddsDetail(row))) drop.add("venue");
  return cols.filter((col) => !drop.has(col.id));
}

function playbookWikiSlug(playbook: string): string {
  return playbook.replace(/\.ya?ml$/i, "");
}

type ReportApiScope = {
  backtestId: string;
  fromMs: number;
  toMs: number;
  /** Experiment/instance chart-series API path (query params appended). */
  chartSeriesPath: string;
};

function StrategyRepeaterItem({
  group,
  live,
  journals,
  defaultOpen,
  selectedId,
  onSelect,
  selectedRow,
  reportApi,
  chartsExtra,
  chartsHold,
  leading,
}: {
  group: StrategyGroup;
  live: LiveState;
  journals: JournalRow[];
  defaultOpen: boolean;
  selectedId: string | null;
  onSelect: (id: string) => void;
  /** Row shown in the action popup. */
  selectedRow: ActivityRow | null;
  reportApi?: ReportApiScope;
  /** Extra charts rendered inside "Charts & decisions" (e.g. monthly returns). */
  chartsExtra?: ReactNode;
  /** Spinner only, until the extra chart series is settled. */
  chartsHold?: boolean;
  /** Rendered above the actions table in the report layout (first group only). */
  leading?: ReactNode;
}) {
  const [actionOpen, setActionOpen] = useState(false);
  const [rangeToolbar, setRangeToolbar] = useState<ReactNode>(null);
  const columns = useMemo(
    () => columnsForRows(live, group.rows, Boolean(reportApi)),
    [live, group.rows, reportApi],
  );
  const chartHeight = useViewportHeight(260, 520, 0.4);
  const chartsPaneRef = useRef<HTMLDivElement>(null);
  const [plotHeight, setPlotHeight] = useState(chartHeight);
  const measureRef = useRef<(flip?: HTMLDetailsElement) => void>(() => {});
  const revealCharts = useCallback(() => {
    measureRef.current();
  }, []);

  useEffect(() => {
    if (!reportApi) return;
    const pane = chartsPaneRef.current;
    if (!pane) return;
    let frame = 0;
    const measure = (flip?: HTMLDetailsElement) => {
      const containerH = pane.clientHeight;
      if (containerH < 80) return;
      const sections = [...pane.querySelectorAll("details")];
      const willOpen = (section: HTMLDetailsElement) => (section === flip ? !section.open : section.open);
      const openCount = sections.filter((section) => willOpen(section)).length;
      if (openCount === 0) return;
      const gap = Number.parseFloat(getComputedStyle(pane).rowGap) || 0;
      const box = (el: Element) => {
        const style = getComputedStyle(el);
        return (
          Number.parseFloat(style.paddingTop) +
          Number.parseFloat(style.paddingBottom) +
          Number.parseFloat(style.borderTopWidth) +
          Number.parseFloat(style.borderBottomWidth)
        );
      };
      let chrome = Math.max(0, sections.length - 1) * gap;
      for (const section of sections) {
        const summary = section.querySelector(":scope > summary");
        if (summary) chrome += summary.getBoundingClientRect().height;
        chrome += box(section);
        if (!willOpen(section)) continue;
        const body = [...section.children].find((el) => el.tagName !== "SUMMARY");
        if (!(body instanceof HTMLElement)) continue;
        chrome += box(body);
        for (const child of body.children) {
          if (!(child instanceof HTMLElement)) continue;
          const style = getComputedStyle(child);
          chrome += Number.parseFloat(style.marginTop) + Number.parseFloat(style.marginBottom);
          const plot = child.hasAttribute("data-chart-plot")
            ? child
            : child.querySelector("[data-chart-plot]");
          if (plot) {
            chrome += Math.max(0, child.getBoundingClientRect().height - plot.getBoundingClientRect().height);
            continue;
          }
          chrome += child.getBoundingClientRect().height;
        }
      }
      const next = Math.floor((containerH - chrome) / openCount);
      if (next < 48) return;
      setPlotHeight((prev) => (Math.abs(prev - next) < 2 ? prev : next));
    };
    measureRef.current = measure;
    const ro = new ResizeObserver(() => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => measure());
    });
    ro.observe(pane);
    const mo = new MutationObserver(() => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => measure());
    });
    mo.observe(pane, { childList: true, subtree: true });
    const onClick = (event: Event) => {
      const summary = (event.target as Element | null)?.closest("summary");
      const details = summary?.parentElement;
      if (!(details instanceof HTMLDetailsElement) || !pane.contains(details)) return;
      flushSync(() => measure(details));
    };
    pane.addEventListener("click", onClick);
    const section = pane.closest("details");
    const onSectionToggle = () => {
      if (!section?.open) return;
      flushSync(() => measure());
    };
    section?.addEventListener("toggle", onSectionToggle);
    measure();
    return () => {
      cancelAnimationFrame(frame);
      ro.disconnect();
      mo.disconnect();
      pane.removeEventListener("click", onClick);
      section?.removeEventListener("toggle", onSectionToggle);
    };
  }, [reportApi, chartsExtra]);
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
    const base = reportApi.chartSeriesPath.split("?")[0] ?? reportApi.chartSeriesPath;
    return `${base}?${q}`;
  }, [reportApi, timeRange]);

  useEffect(() => {
    if (!actionOpen) return;
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape" || e.defaultPrevented) return;
      e.preventDefault();
      setActionOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [actionOpen]);

  function openAction(id: string) {
    onSelect(id);
    setActionOpen(true);
  }

  const decisionsTable = (
    <FilterableTable
      rows={group.rows}
      columns={columns}
      rowKey={(row) => row.orderLinkId}
      emptyMessage="No decisions for this strategy yet."
      className={reportApi ? "min-h-[16rem] max-h-[28rem] xl:max-h-none xl:min-h-[16rem] xl:flex-1" : "min-h-[16rem] max-h-[28rem]"}
      pageSize={PAGE_SIZE}
      framed={false}
      onRowClick={(row) => (reportApi ? openAction(row.orderLinkId) : onSelect(row.orderLinkId))}
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
      <div className="contents">
        <div className="flex min-h-0 min-w-0 flex-col gap-3 overflow-clip xl:h-full">
          {leading ? <div className="shrink-0">{leading}</div> : null}
          <BacktestDetailSection
            title="Actions"
            defaultOpen={defaultOpen}
            fill
            className="min-h-0"
            titleExtra={
              <Badge variant="secondary" className="font-normal tabular-nums">
                {group.rows.length}
              </Badge>
            }
          >
            <div className="relative flex min-h-[16rem] min-w-0 flex-1 flex-col">
              {decisionsTable}
              {actionOpen && selectedRow && (
                <>
                  <button
                    type="button"
                    aria-label="Close action detail"
                    className="absolute inset-0 z-10 cursor-default bg-black/25"
                    onClick={() => setActionOpen(false)}
                  />
                  <div
                    role="dialog"
                    aria-label="Action detail"
                    className="absolute left-1/2 top-1/2 z-20 w-[min(24rem,calc(100%-1rem))] -translate-x-1/2 -translate-y-1/2"
                  >
                    <CandleDetail
                      variant="popup"
                      row={selectedRow}
                      live={live}
                      onClose={() => setActionOpen(false)}
                    />
                  </div>
                </>
              )}
            </div>
          </BacktestDetailSection>
        </div>
        <BacktestDetailSection
          title="Charts & decisions"
          defaultOpen={defaultOpen}
          fill
          className="min-h-0 min-w-0 xl:h-full"
          headerExtra={rangeToolbar}
        >
          <div ref={chartsPaneRef} className="relative flex min-h-0 flex-1 flex-col gap-3 overflow-clip">
            {timeRange && (
              <StrategyCharts
                flat
                fill
                playbook={group.playbook}
                symbol={symbol}
                fromSec={timeRange.fromSec}
                toSec={timeRange.toSec}
                points={points}
                markers={markers}
                chartSeriesPath={chartSeriesPath}
                height={plotHeight}
                extra={chartsExtra}
                hold={chartsHold}
                onReveal={revealCharts}
                onRangeToolbar={setRangeToolbar}
                onMarkerSelect={(time) => {
                  const row = group.rows.find((r) => Math.floor(r.candleClose / 1000) === time);
                  if (row) openAction(row.orderLinkId);
                }}
              />
            )}
          </div>
        </BacktestDetailSection>
      </div>
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
              <StrategyCharts
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
  chartsExtra,
  chartsHold,
  leading,
  reportPending = false,
  headingTitle = "Active Trading Strategies",
  headingDescription = "One panel per deployed playbook. Expand to see portfolio performance and recent decisions.",
  sectionsDefaultOpen,
}: {
  rows: ActivityRow[];
  journals: JournalRow[];
  playbookTitles: Record<string, string>;
  live: LiveState;
  className?: string;
  reportApi?: ReportApiScope;
  /** Rendered inside each reportApi "Charts & decisions" section (after portfolio chart). */
  chartsExtra?: ReactNode;
  /** Report layout only: keep the chart spinner up until the extra series is settled. */
  chartsHold?: boolean;
  /** Report layout only: shown above the actions table in the left column. */
  leading?: ReactNode;
  /** Report layout only: activity rows are still loading. Keeps the two-column shell. */
  reportPending?: boolean;
  /** Ignored when `reportApi` is set (backtest detail embed). */
  headingTitle?: string;
  headingDescription?: string;
  /** When set, overrides the first-group-open default for all sections. */
  sectionsDefaultOpen?: boolean;
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
    // The report layout shows the details beside the table, so it needs no sheet.
    setDetailOpen(!reportApi);
  }

  if (reportApi && rows.length === 0) {
    return (
      <div className={cn("flex min-h-0 flex-1 flex-col", className)}>
        <div className={reportColumnsClass}>
          <div className="flex min-h-0 min-w-0 flex-col gap-3 overflow-clip xl:h-full">
            {leading ? <div className="shrink-0">{leading}</div> : null}
            <BacktestDetailSection title="Actions" defaultOpen={sectionsDefaultOpen ?? true} fill className="min-h-0">
              <div className="flex flex-1 items-center justify-center py-8" role="status">
                {reportPending ? (
                  <Loader2 className="size-5 animate-spin text-muted-foreground" aria-label="Loading actions" />
                ) : (
                  <p className="text-sm text-muted-foreground">No activity for this instance.</p>
                )}
              </div>
            </BacktestDetailSection>
          </div>
          <BacktestDetailSection
            title="Charts & decisions"
            defaultOpen={sectionsDefaultOpen ?? true}
            fill
            className="min-h-0 min-w-0 xl:h-full"
          >
            <div className="flex min-h-40 flex-1 items-center justify-center" role="status" aria-label="Loading charts">
              {reportPending ? (
                <Loader2 className="size-5 animate-spin text-muted-foreground" />
              ) : (
                <p className="text-sm text-muted-foreground">No activity for this instance.</p>
              )}
            </div>
          </BacktestDetailSection>
        </div>
      </div>
    );
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
        <div
          className={cn(
            reportApi ? reportColumnsClass : "flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto",
          )}
        >
          {groups.map((group, index) => (
            <StrategyRepeaterItem
              key={group.playbook}
              group={group}
              live={live}
              journals={journals}
              defaultOpen={sectionsDefaultOpen ?? index === 0}
              selectedId={selectedId}
              onSelect={selectRow}
              selectedRow={selected}
              reportApi={reportApi}
              chartsExtra={chartsExtra}
              chartsHold={chartsHold}
              leading={index === 0 ? leading : undefined}
            />
          ))}
        </div>
      </div>

      {!reportApi && (
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
      )}
    </>
  );
}
