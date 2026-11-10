import { getChartSeries, type ChartSeriesQuery } from "@/lib/experiment-api";

export type { ChartSeriesQuery };
import type { ChartColumns } from "@/lib/chart-series-types";
import {
  ChartPane,
  type ChartSeries,
  ChartSide,
  ChartStyle,
} from "@/lib/gen/tradingcopilot/v1/experiment_chart_pb";

const cache = new Map<string, ChartColumns>();
const inflight = new Map<string, Promise<ChartColumns>>();

function seconds(value: bigint): number {
  return Number(value);
}

export function chartColumns(msg: ChartSeries): ChartColumns {
  return {
    timeSec: msg.timeSec.map(seconds),
    open: msg.open,
    high: msg.high,
    low: msg.low,
    close: msg.close,
    indicators: msg.indicators.map((ind) => ({
      id: ind.id,
      label: ind.label,
      unit: ind.unit,
      pane: ind.pane === ChartPane.OSCILLATOR ? "oscillator" : "price",
      style: ind.style === ChartStyle.HISTOGRAM ? "histogram" : "line",
      reference: ind.reference.length > 0 ? ind.reference : undefined,
      value: ind.value,
    })),
    portfolio: msg.portfolio.map((point) => ({ time: seconds(point.timeSec), value: point.value })),
    markers: msg.markers.flatMap((marker) => {
      if (marker.side !== ChartSide.BUY && marker.side !== ChartSide.SELL) return [];
      return [{ time: seconds(marker.timeSec), side: marker.side === ChartSide.BUY ? "buy" as const : "sell" as const }];
    }),
  };
}

/** The instance a chart series belongs to. */
export type ChartSource = Pick<ChartSeriesQuery, "experimentId" | "instanceId">;

/**
 * A zoomed chart range is fractional unix seconds, the API takes whole seconds. Snap outwards so the window
 * still covers the view, and so nearby fractional ranges share one cache entry.
 */
function snapWindow(q: ChartSeriesQuery): ChartSeriesQuery {
  return { ...q, fromSec: Math.floor(q.fromSec), toSec: Math.ceil(q.toSec) };
}

function cacheKey(q: ChartSeriesQuery): string {
  const w = snapWindow(q);
  return `${w.experimentId}|${w.instanceId}|${w.fromSec}|${w.toSec}|${w.maxPoints ?? 0}`;
}

export function peekChartSeries(q: ChartSeriesQuery): ChartColumns | null {
  return cache.get(cacheKey(q)) ?? null;
}

/** One in-flight request per query. Only a successful decode is stored. */
export function loadChartSeries(q: ChartSeriesQuery): Promise<ChartColumns> {
  q = snapWindow(q);
  const key = cacheKey(q);
  const hit = cache.get(key);
  if (hit) return Promise.resolve(hit);
  const pending = inflight.get(key);
  if (pending) return pending;
  const promise = getChartSeries(q)
    .then((series) => {
      const decoded = chartColumns(series);
      cache.set(key, decoded);
      return decoded;
    })
    .finally(() => {
      inflight.delete(key);
    });
  inflight.set(key, promise);
  return promise;
}

/** Upper bound of candles per chart-series response (the server buckets above it). */
export const CHART_MAX_POINTS_CAP = 4000;

/**
 * Candle budget for one chart response: about two candles per pixel of the window, which keeps the
 * shape of the candles and the indicators. The viewport width stands in for the chart width.
 */
export function chartMaxPoints(viewportWidth: number = typeof window === "undefined" ? 1000 : window.innerWidth): number {
  const budget = Math.round((2 * viewportWidth) / 100) * 100;
  return Math.min(CHART_MAX_POINTS_CAP, Math.max(400, budget));
}

/** Same window the instance charts request: the run, padded by 12 hours. */
export function paddedWindowSec(fromMs: number, toMs: number): { fromSec: number; toSec: number } {
  const padSec = 12 * 3600;
  return { fromSec: Math.floor(fromMs / 1000) - padSec, toSec: Math.ceil(toMs / 1000) + padSec };
}
