import { fromBinary } from "@bufbuild/protobuf";
import type { ChartColumns } from "@/lib/chart-series-types";
import {
  ChartPane,
  ChartSeriesSchema,
  ChartSide,
  ChartStyle,
} from "@/lib/gen/tradingcopilot/v1/experiment_chart_pb";

const cache = new Map<string, ChartColumns>();
const inflight = new Map<string, Promise<ChartColumns>>();

function seconds(value: bigint): number {
  return Number(value);
}

export function decodeChartSeries(bytes: Uint8Array): ChartColumns {
  const msg = fromBinary(ChartSeriesSchema, bytes);
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

export function peekChartSeries(url: string): ChartColumns | null {
  return cache.get(url) ?? null;
}

/** One in-flight request per URL. Only a successful decode is stored. */
export function loadChartSeries(url: string): Promise<ChartColumns> {
  const hit = cache.get(url);
  if (hit) return Promise.resolve(hit);
  const pending = inflight.get(url);
  if (pending) return pending;
  const promise = fetch(url)
    .then(async (res) => {
      if (!res.ok) throw new Error(await res.text());
      const decoded = decodeChartSeries(new Uint8Array(await res.arrayBuffer()));
      cache.set(url, decoded);
      return decoded;
    })
    .finally(() => {
      inflight.delete(url);
    });
  inflight.set(url, promise);
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
export function chartSeriesUrl(base: string, fromMs: number, toMs: number, maxPoints?: number): string {
  const padSec = 12 * 3600;
  return chartSeriesWindowUrl(
    base,
    Math.floor(fromMs / 1000) - padSec,
    Math.ceil(toMs / 1000) + padSec,
    maxPoints,
  );
}

/** The chart-series URL for an exact window in unix seconds. */
export function chartSeriesWindowUrl(base: string, fromSec: number, toSec: number, maxPoints?: number): string {
  const q = new URLSearchParams({ from: String(fromSec), to: String(toSec) });
  if (maxPoints != null && maxPoints > 0) q.set("maxPoints", String(maxPoints));
  const path = base.split("?")[0] ?? base;
  return `${path}?${q}`;
}
