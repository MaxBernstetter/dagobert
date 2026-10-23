"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CandlestickSeries,
  ColorType,
  LineSeries,
  createChart,
  createSeriesMarkers,
  type ISeriesApi,
  type SeriesType,
} from "lightweight-charts";
import type { PortfolioPoint, TradeMarker } from "@/lib/portfolio-series";
import type { ChartBarPayload, ChartIndicatorMeta } from "@/lib/chart-series-types";
import { cn } from "@/lib/utils";

type Props = {
  playbook: string;
  symbol: string;
  fromSec: number;
  toSec: number;
  points: PortfolioPoint[];
  markers: TradeMarker[];
  height?: number;
  /** Override chart-series API (e.g. backtest-scoped route). */
  chartSeriesPath?: string;
};

type LayerVisibility = {
  candles: boolean;
  portfolio: boolean;
  markers: boolean;
  indicators: Record<string, boolean>;
};

function isValidRange(fromSec: number, toSec: number): boolean {
  return Number.isFinite(fromSec) && Number.isFinite(toSec) && toSec > fromSec;
}

/** Extend OHLC series so wheel zoom-out can reach the full requested window (LWC caps zoom at data extent). */
function extendBarsToTimeRange(
  bars: ChartBarPayload[],
  fromSec: number,
  toSec: number,
): ChartBarPayload[] {
  if (!isValidRange(fromSec, toSec) || bars.length === 0) return bars;
  const sorted = [...bars].sort((a, b) => a.time - b.time);
  const out = [...sorted];
  const first = sorted[0];
  const last = sorted[sorted.length - 1];
  if (first.time > fromSec) {
    out.unshift({
      ...first,
      time: fromSec,
      indicators: { ...first.indicators },
    });
  }
  if (last.time < toSec) {
    const c = last.close;
    out.push({
      time: toSec,
      open: c,
      high: c,
      low: c,
      close: c,
      indicators: { ...last.indicators },
    });
  }
  return out;
}

function extendPointsToTimeRange(
  points: PortfolioPoint[],
  fromSec: number,
  toSec: number,
): PortfolioPoint[] {
  if (!isValidRange(fromSec, toSec) || points.length === 0) return points;
  const sorted = [...points].sort((a, b) => a.time - b.time);
  const out = [...sorted];
  const first = sorted[0];
  const last = sorted[sorted.length - 1];
  if (first.time > fromSec) {
    out.unshift({ time: fromSec, value: first.value });
  }
  if (last.time < toSec) {
    out.push({ time: toSec, value: last.value });
  }
  return out;
}

function applyFullTimeRange(
  chart: ReturnType<typeof createChart>,
  fromSec: number,
  toSec: number,
) {
  if (!isValidRange(fromSec, toSec)) {
    chart.timeScale().fitContent();
    return;
  }
  chart.timeScale().setVisibleRange({
    from: fromSec as never,
    to: toSec as never,
  });
}

/** Invisible anchors so the time scale always spans [fromSec, toSec] even with sparse series. */
function addTimeRangeAnchorSeries(
  chart: ReturnType<typeof createChart>,
  fromSec: number,
  toSec: number,
  anchorPrice: number,
) {
  if (!isValidRange(fromSec, toSec) || !Number.isFinite(anchorPrice) || anchorPrice <= 0) {
    return;
  }
  const anchor = chart.addSeries(LineSeries, {
    priceScaleId: "left",
    color: "transparent",
    lineWidth: 1,
    visible: false,
    lastValueVisible: false,
    priceLineVisible: false,
    crosshairMarkerVisible: false,
  });
  anchor.setData([
    { time: fromSec as never, value: anchorPrice },
    { time: toSec as never, value: anchorPrice },
  ]);
}

function formatUtcChartTime(time: number): string {
  const d = new Date(time * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())} UTC`;
}

function ChartLayerToggle({
  active,
  label,
  swatch,
  onClick,
}: {
  active: boolean;
  label: string;
  swatch: React.ReactNode;
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
          : "border-transparent bg-muted/50 text-muted-foreground line-through opacity-70"
      )}
    >
      {swatch}
      {label}
    </button>
  );
}

export function StrategyPortfolioChart({
  playbook,
  symbol,
  fromSec,
  toSec,
  points,
  markers,
  height = 320,
  chartSeriesPath,
}: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [bars, setBars] = useState<ChartBarPayload[]>([]);
  const [indicators, setIndicators] = useState<ChartIndicatorMeta[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState("");
  const [visibility, setVisibility] = useState<LayerVisibility>({
    candles: true,
    portfolio: true,
    markers: true,
    indicators: {},
  });

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setFetchError("");
    (async () => {
      try {
        const q = new URLSearchParams({
          playbook,
          symbol,
          from: String(fromSec),
          to: String(toSec),
        });
        const url = chartSeriesPath ?? `/api/strategy/chart-series?${q}`;
        const res = await fetch(url);
        if (!res.ok) throw new Error(await res.text());
        const data = (await res.json()) as {
          bars: ChartBarPayload[];
          indicators: ChartIndicatorMeta[];
        };
        if (!cancelled) {
          setBars(data.bars);
          setIndicators(data.indicators);
        }
      } catch (err) {
        if (!cancelled) {
          setFetchError(err instanceof Error ? err.message : "Failed to load chart data");
          setBars([]);
          setIndicators([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [playbook, symbol, fromSec, toSec, chartSeriesPath]);

  useEffect(() => {
    setVisibility((v) => ({
      ...v,
      indicators: {
        ...Object.fromEntries(indicators.map((i) => [i.id, true])),
        ...v.indicators,
      },
    }));
  }, [indicators]);

  const toggleCandles = useCallback(() => {
    setVisibility((v) => ({ ...v, candles: !v.candles }));
  }, []);
  const togglePortfolio = useCallback(() => {
    setVisibility((v) => ({ ...v, portfolio: !v.portfolio }));
  }, []);
  const toggleMarkers = useCallback(() => {
    setVisibility((v) => ({ ...v, markers: !v.markers }));
  }, []);
  const toggleIndicator = useCallback((id: string) => {
    setVisibility((v) => ({
      ...v,
      indicators: { ...v.indicators, [id]: !(v.indicators[id] ?? true) },
    }));
  }, []);

  const showLeft = useMemo(() => {
    if (visibility.markers && markers.length > 0) return true;
    if (visibility.candles && bars.length > 0) return true;
    return indicators.some((i) => visibility.indicators[i.id] ?? true);
  }, [visibility, bars.length, indicators, markers.length]);

  const showRight = useMemo(() => {
    if (points.length === 0 && markers.length === 0) return false;
    return visibility.portfolio || (visibility.markers && markers.length > 0);
  }, [visibility, points.length, markers.length]);

  const chartBars = useMemo(
    () => extendBarsToTimeRange(bars, fromSec, toSec),
    [bars, fromSec, toSec],
  );
  const chartPoints = useMemo(
    () => extendPointsToTimeRange(points, fromSec, toSec),
    [points, fromSec, toSec],
  );

  useEffect(() => {
    const node = ref.current;
    if (!node || loading) return;
    const hasEquity = chartPoints.length > 0;
    const hasMarket = chartBars.length > 0;
    if (!hasEquity && !hasMarket) return;

    const anchorPrice =
      chartBars[0]?.close ?? chartPoints[0]?.value ?? 1;

    const dark = document.documentElement.classList.contains("dark");
    const textColor = dark ? "#a1a1aa" : "#71717a";
    const gridColor = dark ? "#27272a" : "#e4e4e7";

    const chart = createChart(node, {
      height,
      layout: { background: { type: ColorType.Solid, color: "transparent" }, textColor },
      localization: {
        timeFormatter: formatUtcChartTime,
      },
      grid: { vertLines: { color: gridColor }, horzLines: { color: gridColor } },
      timeScale: {
        timeVisible: true,
        secondsVisible: false,
        minBarSpacing: 0.05,
        rightOffset: 6,
        fixLeftEdge: false,
        fixRightEdge: false,
      },
      handleScale: {
        axisPressedMouseMove: { time: true, price: true },
        mouseWheel: true,
        pinch: true,
      },
      rightPriceScale: {
        visible: showRight,
        borderVisible: false,
        scaleMargins: { top: 0.1, bottom: 0.15 },
      },
      leftPriceScale: {
        visible: showLeft,
        borderVisible: false,
        scaleMargins: { top: 0.1, bottom: 0.15 },
      },
    });

    let markerHost: ISeriesApi<SeriesType> | null = null;

    addTimeRangeAnchorSeries(chart, fromSec, toSec, anchorPrice);

    if (hasMarket && visibility.candles) {
      const candles = chart.addSeries(CandlestickSeries, {
        priceScaleId: "left",
        upColor: dark ? "#166534" : "#bbf7d0",
        downColor: dark ? "#991b1b" : "#fecaca",
        wickUpColor: dark ? "#22c55e" : "#16a34a",
        wickDownColor: dark ? "#ef4444" : "#dc2626",
        borderVisible: false,
      });
      candles.setData(
        chartBars.map((b) => ({
          time: b.time as never,
          open: b.open,
          high: b.high,
          low: b.low,
          close: b.close,
        }))
      );
      markerHost = candles;
    }

    for (const ind of indicators) {
      if (!(visibility.indicators[ind.id] ?? true)) continue;
      const line = chart.addSeries(LineSeries, {
        priceScaleId: "left",
        color: ind.color,
        lineWidth: 2,
        priceLineVisible: false,
        title: ind.label,
      });
      line.setData(
        chartBars
          .filter((b) => b.indicators[ind.id] != null)
          .map((b) => ({
            time: b.time as never,
            value: b.indicators[ind.id] as number,
          }))
      );
      if (!markerHost) markerHost = line;
    }

    if (hasEquity && visibility.portfolio) {
      const equity = chart.addSeries(LineSeries, {
        priceScaleId: "right",
        color: dark ? "#38bdf8" : "#0284c7",
        lineWidth: 2,
        priceLineVisible: false,
        crosshairMarkerVisible: true,
        title: "Portfolio",
      });
      equity.setData(
        chartPoints.map((p) => ({
          time: p.time as never,
          value: p.value,
        }))
      );
      if (!markerHost) {
        markerHost = equity;
      }
    }

    if (visibility.markers && markers.length > 0 && markerHost) {
      const markerPlugin = createSeriesMarkers(markerHost);
      markerPlugin.setMarkers(
        markers.map((m) => ({
          time: m.time as never,
          position: m.side === "buy" ? "belowBar" : "aboveBar",
          color: m.side === "buy" ? "#22c55e" : "#ef4444",
          shape: m.side === "buy" ? "arrowUp" : "arrowDown",
          text: m.side === "buy" ? "Buy" : "Sell",
        }))
      );
    }

    applyFullTimeRange(chart, fromSec, toSec);

    const resetZoom = () => applyFullTimeRange(chart, fromSec, toSec);
    node.addEventListener("dblclick", resetZoom);

    const onResize = () => chart.applyOptions({ width: node.clientWidth });
    onResize();
    window.addEventListener("resize", onResize);
    return () => {
      node.removeEventListener("dblclick", resetZoom);
      window.removeEventListener("resize", onResize);
      chart.remove();
    };
  }, [
    chartBars,
    indicators,
    chartPoints,
    markers,
    height,
    loading,
    visibility,
    showLeft,
    showRight,
    fromSec,
    toSec,
  ]);

  if (loading) {
    return <p className="text-sm text-muted-foreground py-10 text-center">Loading chart…</p>;
  }
  if (fetchError) {
    return <p className="text-sm text-destructive py-4 text-center">{fetchError}</p>;
  }
  if (points.length === 0 && bars.length === 0) {
    return (
      <p className="text-sm text-muted-foreground rounded-lg bg-muted/40 px-3 py-10 text-center">
        Chart appears once there are KPI bars and at least one filled buy or sell.
      </p>
    );
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">
        Click a series to show or hide it. Scroll to zoom; double-click the chart to fit the full run range.
      </p>
      <div className="flex flex-wrap gap-2">
        {bars.length > 0 && (
          <ChartLayerToggle
            active={visibility.candles}
            label="Candles"
            onClick={toggleCandles}
            swatch={<span className="inline-block size-2 rounded-sm bg-emerald-500/80" aria-hidden />}
          />
        )}
        {indicators.map((ind) => (
          <ChartLayerToggle
            key={ind.id}
            active={visibility.indicators[ind.id] ?? true}
            label={ind.label}
            onClick={() => toggleIndicator(ind.id)}
            swatch={
              <span
                className="inline-block size-2 rounded-full"
                style={{ backgroundColor: ind.color }}
                aria-hidden
              />
            }
          />
        ))}
        {points.length > 0 && (
          <ChartLayerToggle
            active={visibility.portfolio}
            label="Portfolio"
            onClick={togglePortfolio}
            swatch={<span className="inline-block h-0.5 w-3 rounded-full bg-sky-500" aria-hidden />}
          />
        )}
        {markers.length > 0 && (
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
      </div>
      <div ref={ref} className="w-full min-h-[14rem]" />
    </div>
  );
}
