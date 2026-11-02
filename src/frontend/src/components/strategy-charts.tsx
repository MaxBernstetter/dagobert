
import { cloneElement, isValidElement, startTransition, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactElement, type ReactNode } from "react";
import { ChevronRight, Loader2 } from "lucide-react";
import type { PortfolioPoint, TradeMarker } from "@/lib/portfolio-series";
import { emptyChartColumns, type ChartColumns, type ChartIndicatorMeta } from "@/lib/chart-series-types";
import { chartMaxPoints, chartSeriesWindowUrl, loadChartSeries, peekChartSeries } from "@/lib/chart-series-cache";
import { indicatorColor } from "@/components/chart-theme";
import { TimeChart, type ChartHandle, type LayerVisibility, type SeriesPoint, type TimeBand } from "@/components/time-chart";
import { cn } from "@/lib/utils";

type Props = {
  playbook: string;
  symbol: string;
  fromSec: number;
  toSec: number;
  points: PortfolioPoint[];
  markers: TradeMarker[];
  height?: number;
  /** Chart-series API URL (required for candle overlay). */
  chartSeriesPath?: string;
  /** Called with the marker time (unix seconds) when a buy or sell marker is clicked. */
  onMarkerSelect?: (time: number) => void;
  /** Headings and dividers only, for use inside an already bordered section. */
  flat?: boolean;
  /** Open chart sections share the parent's height instead of stacking past it. */
  fill?: boolean;
  /** When set, the time-range control is rendered by the caller (for a section header). */
  onRangeToolbar?: (node: ReactNode | null) => void;
  /** Rendered with the other sections once chart data is ready (for example monthly returns). */
  extra?: ReactNode;
  /** Keep the spinner up and do not mount charts yet (monthly series still loading). */
  hold?: boolean;
  /** Called after the chart sections are in the document, before paint. */
  onReveal?: () => void;
};

type RangeId = "1m" | "3m" | "6m" | "all";

const RANGE_PRESETS: { id: RangeId; label: string; months: number | null }[] = [
  { id: "1m", label: "1M", months: 1 },
  { id: "3m", label: "3M", months: 3 },
  { id: "6m", label: "6M", months: 6 },
  { id: "all", label: "All", months: null },
];

const MONTH_SEC = 30 * 24 * 3600;

function isOscillator(ind: ChartIndicatorMeta): boolean {
  return ind.pane === "oscillator";
}

function isValidRange(fromSec: number, toSec: number): boolean {
  return Number.isFinite(fromSec) && Number.isFinite(toSec) && toSec > fromSec;
}

/** Buy and hold: the start capital invested at the first close, valued at each close. */
function buyHoldPoints(columns: ChartColumns, startCash: number | undefined): SeriesPoint[] {
  if (columns.timeSec.length === 0 || startCash == null || !(startCash > 0)) return [];
  const entry = columns.close.find((value) => value > 0);
  if (entry == null) return [];
  const out: SeriesPoint[] = [];
  for (let i = 0; i < columns.timeSec.length; i++) {
    const close = columns.close[i];
    if (Number.isFinite(close)) out.push({ time: columns.timeSec[i], value: (startCash * close) / entry });
  }
  return out;
}

function rangeFor(id: RangeId, fromSec: number, toSec: number): { from: number; to: number } | null {
  if (!isValidRange(fromSec, toSec)) return null;
  const preset = RANGE_PRESETS.find((p) => p.id === id);
  if (!preset || preset.months == null) return { from: fromSec, to: toSec };
  return { from: Math.max(fromSec, toSec - preset.months * MONTH_SEC), to: toSec };
}

function RangeToolbar({
  range,
  onPick,
}: {
  range: RangeId | null;
  onPick: (id: RangeId) => void;
}) {
  return (
    <div
      className="inline-flex overflow-hidden rounded-md border text-xs"
      role="group"
      aria-label="Time range"
      onClick={(e) => e.stopPropagation()}
    >
      {RANGE_PRESETS.map((p) => (
        <button
          key={p.id}
          type="button"
          onClick={() => onPick(p.id)}
          aria-pressed={range === p.id}
          className={cn(
            "px-2.5 py-1 transition-colors",
            range === p.id ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:bg-muted/50",
          )}
        >
          {p.label}
        </button>
      ))}
    </div>
  );
}


function ChartSpinner({ fill, overlay }: { fill?: boolean; overlay?: boolean }) {
  return (
    <div
      className={cn(
        "flex items-center justify-center",
        overlay ? "absolute inset-0 z-10" : fill ? "min-h-0 flex-1" : "min-h-40",
      )}
      role="status"
      aria-label="Loading charts"
    >
      <Loader2 className="size-5 animate-spin text-muted-foreground" />
    </div>
  );
}

function ChartLayerToggle({
  active,
  label,
  swatch,
  onClick,
}: {
  active: boolean;
  label: string;
  swatch: ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors",
        active
          ? "border-border bg-background text-foreground shadow-sm"
          : "border-transparent bg-muted/50 text-muted-foreground line-through opacity-70",
      )}
    >
      {swatch}
      {label}
    </button>
  );
}

/** A collapsible chart section with a small label and a header. Open by default. */
export function ChartSection({
  title,
  toggles,
  children,
  flat,
  fill,
}: {
  title: string;
  toggles?: ReactNode;
  children: ReactNode;
  flat?: boolean;
  /** Share the column height with sibling sections while this one is open. */
  fill?: boolean;
}) {
  return (
    <details
      open
      className={cn(
        "group",
        flat ? "border-t border-border/50 pt-3" : "rounded-lg border bg-card",
        fill && "shrink-0",
      )}
    >
      <summary
        className={cn(
          "flex shrink-0 cursor-pointer list-none items-center gap-2 [&::-webkit-details-marker]:hidden",
          flat ? "py-1" : "px-3 py-2",
        )}
      >
        <ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90" aria-hidden />
        <h4 className="min-w-0 text-sm font-medium">{title}</h4>
      </summary>
      <div
        className={cn(
          "space-y-2",
          flat ? "pb-1 pt-2" : "border-t px-3 pb-3 pt-2",
        )}
      >
        {toggles ? <div className="flex shrink-0 flex-wrap gap-2">{toggles}</div> : null}
        <div data-chart-slot>{children}</div>
      </div>
    </details>
  );
}


export function StrategyCharts({
  playbook,
  symbol,
  fromSec,
  toSec,
  points,
  markers,
  height = 320,
  chartSeriesPath,
  onMarkerSelect,
  flat,
  fill,
  onRangeToolbar,
  extra,
  hold = false,
  onReveal,
}: Props) {
  // The instance's chart series for the whole window, budgeted to the viewport: buy and hold and the
  // first paint use it. A zoom or range preset loads the visible window with the same budget.
  const [budget] = useState(() => chartMaxPoints());
  const basePath = chartSeriesPath ? chartSeriesPath.split("?")[0] ?? chartSeriesPath : undefined;
  const fullPath = basePath ? chartSeriesWindowUrl(basePath, fromSec, toSec, budget) : undefined;
  const instanceKey = basePath ? `${basePath}|${fromSec}|${toSec}` : "";
  const [dataWindow, setDataWindow] = useState<{ from: number; to: number } | null>(null);
  const dataPath = dataWindow && basePath ? chartSeriesWindowUrl(basePath, dataWindow.from, dataWindow.to, budget) : fullPath;

  const [columns, setColumns] = useState<ChartColumns>(() => (fullPath ? peekChartSeries(fullPath) : null) ?? emptyChartColumns);
  const [fullColumns, setFullColumns] = useState<ChartColumns>(() => (fullPath ? peekChartSeries(fullPath) : null) ?? emptyChartColumns);
  const [loading, setLoading] = useState(() => (fullPath ? peekChartSeries(fullPath) == null : false));
  const [fetchError, setFetchError] = useState("");
  const [range, setRange] = useState<RangeId | null>("all");
  const [visibility, setVisibility] = useState<LayerVisibility>({
    candles: true,
    portfolio: true,
    buyHold: true,
    markers: true,
    indicators: {},
  });
  const [activeKey, setActiveKey] = useState(instanceKey);
  if (activeKey !== instanceKey) {
    // Another instance or window of the experiment: its data is not this one's, drop it.
    setActiveKey(instanceKey);
    setDataWindow(null);
    setFetchError("");
    setColumns(emptyChartColumns);
    setFullColumns(emptyChartColumns);
    setLoading(Boolean(instanceKey));
  }

  // Full-range series: the reference for buy and hold, so zooming does not move that line.
  useEffect(() => {
    if (!fullPath) return;
    let cancelled = false;
    loadChartSeries(fullPath)
      .then((data) => {
        if (!cancelled) startTransition(() => setFullColumns(data));
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [fullPath]);

  useEffect(() => {
    let cancelled = false;
    if (!dataPath) {
      setColumns(emptyChartColumns);
      setLoading(false);
      setFetchError("");
      return;
    }
    const cached = peekChartSeries(dataPath);
    if (cached) {
      startTransition(() => setColumns(cached));
      setLoading(false);
    }
    setFetchError("");
    loadChartSeries(dataPath)
      .then((data) => {
        // Keep the previous window on screen until the new one is here: no spinner between zooms.
        if (!cancelled) startTransition(() => setColumns(data));
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setFetchError(err instanceof Error ? err.message : "Failed to load chart data");
          setColumns(emptyChartColumns);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [dataPath]);

  const indicators = columns.indicators;
  useEffect(() => {
    setVisibility((v) => ({
      ...v,
      indicators: { ...Object.fromEntries(indicators.map((i) => [i.id, true])), ...v.indicators },
    }));
  }, [indicators]);

  const seriesPoints = columns.portfolio.length > 0 ? columns.portfolio : points;
  const seriesMarkers = columns.markers.length > 0 ? columns.markers : markers;
  const chartPoints = useMemo(
    () => [...seriesPoints].sort((a, b) => a.time - b.time),
    [seriesPoints],
  );
  const buyHold = useMemo(() => buyHoldPoints(fullColumns, chartPoints[0]?.value), [fullColumns, chartPoints]);
  const priceIndicators = useMemo(() => indicators.filter((i) => !isOscillator(i)), [indicators]);
  const oscillators = useMemo(() => indicators.filter(isOscillator), [indicators]);
  const [painted, setPainted] = useState<{ path: string | undefined; ids: Set<string> }>({
    path: chartSeriesPath,
    ids: new Set(),
  });
  const paintedIds = painted.path === chartSeriesPath ? painted.ids : new Set<string>();
  const markPainted = useCallback((id: string) => {
    setPainted((prev) => {
      const ids = prev.path === chartSeriesPath ? new Set(prev.ids) : new Set<string>();
      if (ids.has(id) && prev.path === chartSeriesPath) return prev;
      ids.add(id);
      return { path: chartSeriesPath, ids };
    });
  }, [chartSeriesPath]);
  const expectedIds = useMemo(() => {
    const ids: string[] = [];
    if (columns.timeSec.length > 0) {
      ids.push("price");
      if (oscillators.length > 0) ids.push("oscillators");
    }
    if (chartPoints.length > 0 || columns.timeSec.length > 0) ids.push("portfolio");
    if (extra) ids.push("monthly");
    return ids;
  }, [columns.timeSec.length, oscillators.length, chartPoints.length, extra]);
  const revealed = expectedIds.length > 0 && expectedIds.every((id) => paintedIds.has(id));

  // Shared time axis: the charts register here; a range change in one is applied to all the others.
  const chartsRef = useRef(new Map<string, ChartHandle>());
  const syncingRef = useRef(false);
  const rangeRef = useRef(range);
  rangeRef.current = range;
  const boundsRef = useRef({ fromSec, toSec });
  boundsRef.current = { fromSec, toSec };
  const [timeBand, setTimeBand] = useState<TimeBand | null>(null);

  const onReady = useCallback((id: string, chart: ChartHandle | null) => {
    if (chart) {
      chartsRef.current.set(id, chart);
      const selected = rangeRef.current;
      if (!selected) return;
      const r = rangeFor(selected, boundsRef.current.fromSec, boundsRef.current.toSec);
      if (r) chart.setWindow(r.from, r.to);
    } else {
      chartsRef.current.delete(id);
    }
  }, []);

  // A zoom changes the window the candles are loaded for. The data follows after a short pause
  // (wheel zoom fires on every step), the view moves at once.
  const windowTimer = useRef<number | undefined>(undefined);
  const loadWindow = useCallback((from: number, to: number, now: boolean) => {
    window.clearTimeout(windowTimer.current);
    const full = from <= fromSec && to >= toSec;
    const next = full ? null : { from, to };
    if (now) {
      setDataWindow(next);
      return;
    }
    windowTimer.current = window.setTimeout(() => setDataWindow(next), 250);
  }, [fromSec, toSec]);
  useEffect(() => () => window.clearTimeout(windowTimer.current), []);

  const onRange = useCallback((id: string, from: number, to: number) => {
    if (syncingRef.current) return;
    loadWindow(from, to, false);
    syncingRef.current = true;
    try {
      for (const [other, chart] of chartsRef.current) {
        if (other !== id) chart.setWindow(from, to);
      }
    } finally {
      syncingRef.current = false;
    }
  }, [loadWindow]);

  const applyWindow = useCallback((from: number, to: number) => {
    syncingRef.current = true;
    try {
      for (const chart of chartsRef.current.values()) chart.setWindow(from, to);
    } finally {
      syncingRef.current = false;
    }
  }, []);


  const pickRange = useCallback((id: RangeId) => {
    setRange(id);
    const r = rangeFor(id, fromSec, toSec);
    if (!r) return;
    applyWindow(r.from, r.to);
    loadWindow(r.from, r.to, true);
  }, [applyWindow, fromSec, toSec, loadWindow]);
  const zoomToBand = useCallback((from: number, to: number) => {
    setRange(null);
    setTimeBand(null);
    applyWindow(from, to);
    loadWindow(from, to, true);
  }, [applyWindow, loadWindow]);
  const resetAll = useCallback(() => {
    setRange("all");
    applyWindow(fromSec, toSec);
    loadWindow(fromSec, toSec, true);
  }, [applyWindow, fromSec, toSec, loadWindow]);

  useEffect(() => {
    if (!onRangeToolbar) return;
    if (loading || hold || !revealed) {
      onRangeToolbar(null);
      return;
    }
    onRangeToolbar(<RangeToolbar range={range} onPick={pickRange} />);
    return () => onRangeToolbar(null);
  }, [onRangeToolbar, range, pickRange, loading, hold, revealed]);

  useLayoutEffect(() => {
    if (loading || hold) return;
    onReveal?.();
  }, [loading, hold, onReveal]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented || !timeBand) return;
      event.preventDefault();
      setTimeBand(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [timeBand]);

  const toggleCandles = () => setVisibility((v) => ({ ...v, candles: !v.candles }));
  const togglePortfolio = () => setVisibility((v) => ({ ...v, portfolio: !v.portfolio }));
  const toggleBuyHold = () => setVisibility((v) => ({ ...v, buyHold: !v.buyHold }));
  const toggleMarkers = () => setVisibility((v) => ({ ...v, markers: !v.markers }));
  const toggleIndicator = (id: string) =>
    setVisibility((v) => ({ ...v, indicators: { ...v.indicators, [id]: !(v.indicators[id] ?? true) } }));

  const extraSection = extra ? (
    <ChartSection flat={flat} fill={fill} title="Monthly returns">
      {isValidElement(extra)
        ? cloneElement(extra as ReactElement<{ height?: number; onPainted?: () => void }>, {
            height,
            onPainted: () => markPainted("monthly"),
          })
        : extra}
    </ChartSection>
  ) : null;

  if (fetchError) {
    return (
      <div className={cn(fill && "contents")}>
        <p className="py-4 text-center text-sm text-destructive">{fetchError}</p>
        {extraSection}
      </div>
    );
  }
  if (loading || hold) {
    return <ChartSpinner fill={fill} />;
  }
  if (chartPoints.length === 0 && columns.timeSec.length === 0) {
    return (
      <div className={cn(fill && "contents")}>
        <p className="rounded-lg bg-muted/40 px-3 py-10 text-center text-sm text-muted-foreground">
          Chart appears once there are KPI bars and at least one filled buy or sell.
        </p>
        {extraSection}
      </div>
    );
  }

  const oscHeight = fill ? height : Math.max(140, Math.round(height * 0.5), 96 * oscillators.length);
  const portfolioHeight = fill ? height : Math.max(160, Math.round(height * 0.65));

  return (
    <div className={cn(fill ? "contents" : "relative space-y-3")}>
      {!revealed && <ChartSpinner fill={fill} overlay />}
      <div
        key={chartSeriesPath ?? ""}
        aria-hidden={!revealed}
        className={cn(!revealed && "invisible", fill ? "contents" : "space-y-3")}
      >
      {!onRangeToolbar && (
        <div className="flex shrink-0 justify-end">
          <RangeToolbar range={range} onPick={pickRange} />
        </div>
      )}

      {columns.timeSec.length > 0 && (
        <ChartSection
          flat={flat}
          fill={fill}
          title="Numeric technical indicators"
          toggles={
            <>
              <ChartLayerToggle
                active={visibility.candles}
                label="Candles"
                onClick={toggleCandles}
                swatch={<span className="inline-block size-2 rounded-sm bg-emerald-500/80" aria-hidden />}
              />
              {priceIndicators.map((ind, index) => (
                <ChartLayerToggle
                  key={ind.id}
                  active={visibility.indicators[ind.id] ?? true}
                  label={ind.label}
                  onClick={() => toggleIndicator(ind.id)}
                  swatch={<span className="inline-block size-2 rounded-full" style={{ backgroundColor: indicatorColor(index) }} aria-hidden />}
                />
              ))}
            </>
          }
        >
          <TimeChart
            id="price"
            kind="indicators"
            columns={columns}
            points={chartPoints}
            buyHold={buyHold}
            indicators={indicators}
            markers={seriesMarkers}
            visibility={visibility}
            height={height}
            fromSec={fromSec}
            toSec={toSec}
            timeBand={timeBand}
            onTimeBand={setTimeBand}
            onTimeZoom={zoomToBand}
            onReady={onReady}
            onRange={onRange}
            onReset={resetAll}
            onPainted={() => markPainted("price")}
          />
        </ChartSection>
      )}

      {columns.timeSec.length > 0 && oscillators.length > 0 && (
        <ChartSection
          flat={flat}
          fill={fill}
          title="Oscillators"
          toggles={
            <>
              {oscillators.map((ind, index) => (
                <ChartLayerToggle
                  key={ind.id}
                  active={visibility.indicators[ind.id] ?? true}
                  label={ind.label}
                  onClick={() => toggleIndicator(ind.id)}
                  swatch={
                    ind.style === "histogram" ? (
                      <span className="inline-flex items-end gap-px" aria-hidden>
                        <span className="inline-block h-2.5 w-1 bg-emerald-500/70" />
                        <span className="inline-block h-1.5 w-1 bg-red-500/70" />
                      </span>
                    ) : (
                      <span className="inline-block size-2 rounded-full" style={{ backgroundColor: indicatorColor(index) }} aria-hidden />
                    )
                  }
                />
              ))}
            </>
          }
        >
          <TimeChart
            id="oscillators"
            kind="oscillators"
            columns={columns}
            points={chartPoints}
            buyHold={buyHold}
            indicators={indicators}
            markers={seriesMarkers}
            visibility={visibility}
            height={oscHeight}
            fromSec={fromSec}
            toSec={toSec}
            timeBand={timeBand}
            onTimeBand={setTimeBand}
            onTimeZoom={zoomToBand}
            onReady={onReady}
            onRange={onRange}
            onReset={resetAll}
            onPainted={() => markPainted("oscillators")}
          />
        </ChartSection>
      )}

      <ChartSection
        flat={flat}
        fill={fill}
        title="Portfolio with action marks"
        toggles={
          <>
            {chartPoints.length > 0 && (
              <ChartLayerToggle
                active={visibility.portfolio}
                label="Portfolio"
                onClick={togglePortfolio}
                swatch={<span className="inline-block h-0.5 w-3 rounded-full bg-sky-500" aria-hidden />}
              />
            )}
            {chartPoints.length > 0 && buyHold.length > 0 && (
              <ChartLayerToggle
                active={visibility.buyHold}
                label="Buy & hold"
                onClick={toggleBuyHold}
                swatch={<span className="inline-block h-0 w-3 border-t border-dashed border-muted-foreground" aria-hidden />}
              />
            )}
            {seriesMarkers.length > 0 && (
              <ChartLayerToggle
                active={visibility.markers}
                label="Buy / sell"
                onClick={toggleMarkers}
                swatch={
                  <span className="inline-flex gap-px" aria-hidden>
                    <span className="text-[10px] text-emerald-500">▲</span>
                    <span className="text-[10px] text-red-500">▼</span>
                  </span>
                }
              />
            )}
          </>
        }
      >
        <TimeChart
          id="portfolio"
          kind="portfolio"
          columns={columns}
          points={chartPoints}
          buyHold={buyHold}
          indicators={indicators}
          markers={seriesMarkers}
          visibility={visibility}
          height={portfolioHeight}
          fromSec={fromSec}
          toSec={toSec}
          timeBand={timeBand}
          onTimeBand={setTimeBand}
          onTimeZoom={zoomToBand}
          onReady={onReady}
          onRange={onRange}
          onReset={resetAll}
          onMarkerSelect={onMarkerSelect}
          onPainted={() => markPainted("portfolio")}
        />
      </ChartSection>
      {extraSection}
      </div>
    </div>
  );
}
