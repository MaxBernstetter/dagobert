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

/** Same window the instance charts request: the run, padded by 12 hours. */
export function chartSeriesUrl(base: string, fromMs: number, toMs: number): string {
  const padSec = 12 * 3600;
  const q = new URLSearchParams({
    from: String(Math.floor(fromMs / 1000) - padSec),
    to: String(Math.ceil(toMs / 1000) + padSec),
  });
  const path = base.split("?")[0] ?? base;
  return `${path}?${q}`;
}
