"use client";

import { useEffect, useRef, useState } from "react";
import type { ECharts } from "@/components/echarts-setup";
import { init } from "@/components/echarts-setup";
import {
  AXIS_TITLE_MIN_PX,
  chartFontFamily,
  formatAxisNumber,
  formatUsdc,
  formatUtcChartTime,
  formatUtcTick,
  indicatorColor,
  readChartTheme,
  type ChartTheme,
} from "@/components/chart-theme";
import type { ChartColumnIndicator, ChartColumns, ChartIndicatorMeta } from "@/lib/chart-series-types";
import type { PortfolioPoint, TradeMarker } from "@/lib/portfolio-series";

export type TimeBand = { from: number; to: number };
export type ValueBand = { axis: number; min: number; max: number };
export type ChartHandle = { setWindow: (fromSec: number, toSec: number) => void };
export type SeriesPoint = { time: number; value: number };

export type LayerVisibility = {
  candles: boolean;
  portfolio: boolean;
  buyHold: boolean;
  markers: boolean;
  indicators: Record<string, boolean>;
};

type ChartKind = "indicators" | "oscillators" | "portfolio";

type Hover = { time: number; rows: { label: string; value: string }[]; marker?: "Buy" | "Sell" };

const BAR_SEC = 3 * 3600;

function isOscillator(ind: ChartIndicatorMeta): boolean {
  return ind.pane === "oscillator";
}

function finite(values: number[]): number[] {
  return values.filter((v) => Number.isFinite(v));
}

function samples(times: number[], values: number[]): [number, number][] {
  const out: [number, number][] = [];
  const n = Math.min(times.length, values.length);
  for (let i = 0; i < n; i++) {
    if (Number.isFinite(values[i])) out.push([times[i] * 1000, values[i]]);
  }
  return out;
}

function columnOf(columns: ChartColumns, id: string): number[] {
  return columns.indicators.find((ind) => ind.id === id)?.value ?? [];
}

/** Price-pane series far from the candles (ATR is hundreds, price is tens of thousands) use a side scale. */
function indicatorNearCandles(columns: ChartColumns, ind: ChartColumnIndicator): boolean {
  const values = finite(ind.value);
  const lows = finite(columns.low);
  const highs = finite(columns.high);
  if (values.length === 0 || lows.length === 0 || highs.length === 0) return true;
  const candles = robustRange([...lows, ...highs], false);
  const series = robustRange(values, false);
  return series.max >= candles.min && series.min <= candles.max;
}

function robustRange(values: number[], includeZero: boolean): { min: number; max: number } {
  const v = values.filter(Number.isFinite).sort((a, b) => a - b);
  if (v.length === 0) return { min: -1, max: 1 };
  const at = (p: number) => v[Math.min(v.length - 1, Math.floor(p * (v.length - 1)))];
  let lo = at(0.02);
  let hi = at(0.98);
  if (includeZero) {
    lo = Math.min(lo, 0);
    hi = Math.max(hi, 0);
  }
  if (hi - lo < 1e-9) {
    lo -= 1;
    hi += 1;
  }
  const pad = (hi - lo) * 0.1;
  return { min: lo - pad, max: hi + pad };
}

function equityAt(points: PortfolioPoint[], time: number): number {
  let value = points[0]?.value ?? 0;
  for (const p of points) {
    if (p.time > time) break;
    value = p.value;
  }
  return value;
}

function nearestTime(times: number[], target: number): number | null {
  if (times.length === 0) return null;
  let best = times[0];
  let bestD = Math.abs(best - target);
  for (const t of times) {
    const d = Math.abs(t - target);
    if (d < bestD) {
      best = t;
      bestD = d;
    }
  }
  return best;
}

function clampWindow(from: number, to: number, min: number, max: number): TimeBand {
  const a = Math.max(min, Math.min(from, to));
  const b = Math.min(max, Math.max(from, to));
  if (!(b - a > 1)) return { from: min, to: max };
  return { from: a, to: b };
}

function sameWindow(a: TimeBand, b: TimeBand): boolean {
  return Math.abs(a.from - b.from) < 0.5 && Math.abs(a.to - b.to) < 0.5;
}

function readWindow(chart: ECharts, fromSec: number, toSec: number): TimeBand {
  const zoom = chart.getOption().dataZoom as { start?: number; end?: number; startValue?: number; endValue?: number }[] | undefined;
  const dz = zoom?.[0];
  if (!dz) return { from: fromSec, to: toSec };
  if (typeof dz.startValue === "number" && typeof dz.endValue === "number" && dz.endValue > dz.startValue) {
    return clampWindow(dz.startValue / 1000, dz.endValue / 1000, fromSec, toSec);
  }
  const span = toSec - fromSec;
  const start = dz.start ?? 0;
  const end = dz.end ?? 100;
  return clampWindow(fromSec + (start / 100) * span, fromSec + (end / 100) * span, fromSec, toSec);
}

function gridIndexAt(y: number, count: number, height: number): number {
  if (count <= 1) return 0;
  const titleGap = height >= AXIS_TITLE_MIN_PX ? 36 : 22;
  const gap = 8;
  const top = 8;
  const inner = Math.max(24, height - top - titleGap - gap * (count - 1));
  const rowH = inner / count;
  for (let i = 0; i < count; i++) {
    const start = top + i * (rowH + gap);
    if (y >= start && y <= start + rowH) return i;
  }
  return 0;
}

function pixelOnAxis(chart: ECharts, x: number, y: number, axis: number): [number, number] | null {
  const raw = chart.convertFromPixel({ xAxisIndex: axis, yAxisIndex: axis }, [x, y]);
  if (!Array.isArray(raw)) return null;
  const ms = Number(raw[0]);
  const value = Number(raw[1]);
  if (!Number.isFinite(ms) || !Number.isFinite(value)) return null;
  return [ms, value];
}

function markArea(theme: ChartTheme, timeBand: TimeBand | null, valueBand: { min: number; max: number } | null) {
  const data: [{ xAxis?: number; yAxis?: number }, { xAxis?: number; yAxis?: number }][] = [];
  if (timeBand) data.push([{ xAxis: timeBand.from * 1000 }, { xAxis: timeBand.to * 1000 }]);
  if (valueBand) data.push([{ yAxis: valueBand.min }, { yAxis: valueBand.max }]);
  return { silent: true, itemStyle: { color: theme.highlight }, data };
}

type BuildInput = {
  kind: ChartKind;
  theme: ChartTheme;
  font: string;
  columns: ChartColumns;
  points: PortfolioPoint[];
  buyHold: SeriesPoint[];
  indicators: ChartIndicatorMeta[];
  markers: TradeMarker[];
  visibility: LayerVisibility;
  height: number;
  fromSec: number;
  toSec: number;
  window: TimeBand;
  timeBand: TimeBand | null;
  valueBand: ValueBand | null;
};

function axisChrome(theme: ChartTheme, spanSec: number, showLabels: boolean) {
  return {
    axisLine: { lineStyle: { color: theme.axisLine } },
    axisTick: { show: showLabels, lineStyle: { color: theme.axisLine } },
    axisLabel: {
      show: showLabels,
      color: theme.text,
      fontSize: 11,
      hideOverlap: true,
      formatter: (value: number) => formatUtcTick(value, spanSec),
    },
    splitLine: { show: true, lineStyle: { color: theme.grid } },
  };
}

function buildOption(input: BuildInput) {
  const { theme, kind, height, fromSec, toSec, window: win } = input;
  const spanSec = Math.max(1, win.to - win.from);
  const showXTitle = height >= AXIS_TITLE_MIN_PX;
  const ms = (sec: number) => sec * 1000;
  const titleGap = showXTitle ? 36 : 22;

  const xBase = (index: number, showLabels: boolean) => ({
    type: "time" as const,
    gridIndex: index,
    min: fromSec * 1000,
    max: toSec * 1000,
    name: showLabels && showXTitle ? "UTC" : "",
    nameLocation: "middle" as const,
    nameGap: 24,
    nameTextStyle: { color: theme.text, fontSize: 11 },
    ...axisChrome(theme, spanSec, showLabels),
  });

  const yBase = (index: number, name: string, extra: Record<string, unknown>) => {
    const showName = name !== "" && height >= AXIS_TITLE_MIN_PX;
    return {
      type: "value" as const,
      gridIndex: index,
      name: showName ? name : "",
      nameLocation: "middle" as const,
      nameRotate: extra.position === "right" ? -90 : 90,
      nameGap: showName ? 46 : 8,
      nameTextStyle: { color: theme.text, fontSize: 11 },
      scale: true,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: theme.text, fontSize: 11, formatter: (value: number) => formatAxisNumber(value) },
      splitLine: { show: true, lineStyle: { color: theme.grid } },
      ...extra,
    };
  };

  const dataZoom = [{
    type: "inside" as const,
    filterMode: "weakFilter" as const,
    startValue: win.from * 1000,
    endValue: win.to * 1000,
    zoomOnMouseWheel: true,
    moveOnMouseMove: false,
    moveOnMouseWheel: false,
    throttle: 40,
    xAxisIndex: [] as number[],
  }];

  const series: Record<string, unknown>[] = [];
  const grids: Record<string, unknown>[] = [];
  const xAxes: Record<string, unknown>[] = [];
  const yAxes: Record<string, unknown>[] = [];

  const priceIndicators = kind === "indicators"
    ? input.columns.indicators.filter((ind) => !isOscillator(ind))
    : [];
  const onPriceScale = new Set(priceIndicators.filter((ind) => indicatorNearCandles(input.columns, ind)).map((ind) => ind.id));
  const sideScale = priceIndicators.some((ind) => !onPriceScale.has(ind.id) && (input.visibility.indicators[ind.id] ?? true));
  const left = kind === "portfolio" ? 16 : height >= AXIS_TITLE_MIN_PX ? 72 : 52;
  const right = kind === "portfolio"
    ? (height >= AXIS_TITLE_MIN_PX ? 72 : 56)
    : sideScale
      ? (height >= AXIS_TITLE_MIN_PX ? 56 : 44)
      : 12;

  if (kind === "oscillators") {
    const osc = input.columns.indicators.filter(isOscillator);
    const n = Math.max(1, osc.length);
    const gap = 8;
    const inner = Math.max(24, height - 8 - titleGap - gap * (n - 1));
    const rowH = inner / n;
    osc.forEach((ind, i) => {
      const includeZero = ind.style === "histogram" || (ind.reference ?? []).includes(0);
      const showYName = rowH >= AXIS_TITLE_MIN_PX;
      grids.push({ left, right, top: 8 + i * (rowH + gap), height: rowH });
      xAxes.push(xBase(i, i === osc.length - 1));
      yAxes.push(yBase(i, showYName ? (ind.unit || ind.label) : "", includeZero
        ? { min: (extent: { min: number }) => Math.min(0, extent.min), max: (extent: { max: number }) => Math.max(0, extent.max) }
        : {}));
      dataZoom[0].xAxisIndex.push(i);
      const visible = input.visibility.indicators[ind.id] ?? true;
      const area = markArea(
        theme,
        input.timeBand,
        input.valueBand?.axis === i ? input.valueBand : null,
      );
      if (ind.style === "histogram") {
        series.push({
          type: "bar",
          name: ind.label,
          xAxisIndex: i,
          yAxisIndex: i,
          data: samples(input.columns.timeSec, ind.value),
          itemStyle: {
            color: (p: { value: number[] }) => ((p.value[1] ?? 0) >= 0 ? theme.positive : theme.negative),
            opacity: visible ? 1 : 0,
          },
          markLine: referenceLines(theme, ind.reference),
          markArea: area,
          emphasis: { disabled: true },
        });
      } else {
        series.push({
          type: "line",
          name: ind.label,
          xAxisIndex: i,
          yAxisIndex: i,
          showSymbol: false,
          data: samples(input.columns.timeSec, ind.value),
          lineStyle: { width: 1.5, color: indicatorColor(i, theme), opacity: visible ? 1 : 0 },
          itemStyle: { color: indicatorColor(i, theme) },
          markLine: referenceLines(theme, ind.reference),
          markArea: area,
          emphasis: { disabled: true },
        });
      }
    });
  } else if (kind === "portfolio") {
    grids.push({ left, right, top: 8, bottom: titleGap });
    xAxes.push(xBase(0, true));
    yAxes.push(yBase(0, "USDC", { position: "right" }));
    dataZoom[0].xAxisIndex.push(0);
    const area = markArea(theme, input.timeBand, input.valueBand?.axis === 0 ? input.valueBand : null);
    if (input.points.length > 0) {
      series.push({
        type: "line",
        name: "Portfolio",
        showSymbol: false,
        data: input.points.map((p) => [ms(p.time), p.value]),
        lineStyle: { width: 1.75, color: theme.portfolio, opacity: input.visibility.portfolio ? 1 : 0 },
        itemStyle: { color: theme.portfolio },
        markArea: area,
        markPoint: input.visibility.markers ? buySellMarks(theme, input.markers, input.points) : undefined,
        emphasis: { disabled: true },
      });
    }
    if (input.points.length > 0 && input.buyHold.length > 0) {
      series.push({
        type: "line",
        name: "Buy & hold",
        showSymbol: false,
        data: input.buyHold.map((p) => [ms(p.time), p.value]),
        lineStyle: {
          width: 1.25,
          type: "dashed",
          color: theme.hold,
          opacity: input.visibility.buyHold ? 1 : 0,
        },
        itemStyle: { color: theme.hold },
        emphasis: { disabled: true },
      });
    }
  } else {
    grids.push({ left, right, top: 8, bottom: titleGap });
    xAxes.push(xBase(0, true));
    yAxes.push(yBase(0, "Price", {}));
    if (sideScale) {
      yAxes.push(yBase(0, "", { position: "right", splitLine: { show: false } }));
    }
    dataZoom[0].xAxisIndex.push(0);
    const area = markArea(theme, input.timeBand, input.valueBand?.axis === 0 ? input.valueBand : null);
    series.push({
      type: "candlestick",
      name: "Close",
      data: input.columns.timeSec.map((time, i) => [time * 1000, input.columns.open[i], input.columns.close[i], input.columns.low[i], input.columns.high[i]]),
      itemStyle: {
        color: theme.candleUpFill,
        color0: theme.candleDownFill,
        borderColor: theme.candleUp,
        borderColor0: theme.candleDown,
        opacity: input.visibility.candles ? 1 : 0,
      },
      markArea: area,
      emphasis: { disabled: true },
    });
    priceIndicators.forEach((ind, index) => {
      const visible = input.visibility.indicators[ind.id] ?? true;
      const color = indicatorColor(index, theme);
      const ownScale = sideScale && !onPriceScale.has(ind.id);
      series.push({
        type: "line",
        name: ind.label,
        yAxisIndex: ownScale ? 1 : 0,
        showSymbol: false,
        data: samples(input.columns.timeSec, columnOf(input.columns, ind.id)),
        lineStyle: { width: 1.5, color, opacity: visible ? 1 : 0 },
        itemStyle: { color },
        emphasis: { disabled: true },
      });
    });
  }

  return {
    animation: false,
    backgroundColor: "transparent",
    textStyle: { color: theme.text, fontFamily: input.font },
    grid: grids,
    xAxis: xAxes,
    yAxis: yAxes,
    dataZoom,
    series,
    axisPointer: {
      show: true,
      type: "line",
      label: { show: false },
      lineStyle: { color: theme.axisLine, type: "dashed" },
      link: [{ xAxisIndex: "all" }],
    },
  };
}

function referenceLines(theme: ChartTheme, levels: number[] | undefined) {
  if (!levels?.length) return undefined;
  return {
    silent: true,
    symbol: "none",
    label: { show: false },
    lineStyle: { type: "dashed", color: theme.hold, opacity: 0.7 },
    data: levels.map((level) => ({ yAxis: level })),
  };
}

function buySellMarks(theme: ChartTheme, markers: TradeMarker[], points: PortfolioPoint[]) {
  return {
    symbol: "triangle",
    symbolSize: 9,
    label: { show: false },
    data: markers.map((m) => ({
      name: String(m.time),
      coord: [m.time * 1000, equityAt(points, m.time)],
      symbolRotate: m.side === "buy" ? 0 : 180,
      symbolOffset: m.side === "buy" ? [0, 8] : [0, -8],
      itemStyle: { color: m.side === "buy" ? theme.buy : theme.sell },
    })),
  };
}

function hoverAt(
  kind: ChartKind,
  time: number,
  columns: ChartColumns,
  points: PortfolioPoint[],
  buyHold: SeriesPoint[],
  indicators: ChartIndicatorMeta[],
  markers: TradeMarker[],
  visibility: LayerVisibility,
): Hover | null {
  const times = kind === "portfolio" ? points.map((p) => p.time) : columns.timeSec;
  const nearest = nearestTime(times, time);
  if (nearest == null || Math.abs(nearest - time) > BAR_SEC) return null;
  const rows: Hover["rows"] = [];
  const num = (v: number) => v.toLocaleString(undefined, { maximumFractionDigits: 2 });
  const index = columns.timeSec.indexOf(nearest);
  if (kind === "indicators") {
    if (index >= 0 && visibility.candles) rows.push({ label: "Close", value: num(columns.close[index]) });
    if (index >= 0) {
      columns.indicators.filter((ind) => !isOscillator(ind)).forEach((ind) => {
        if (!(visibility.indicators[ind.id] ?? true)) return;
        const value = ind.value[index];
        if (Number.isFinite(value)) rows.push({ label: ind.label, value: num(value) });
      });
    }
  } else if (kind === "oscillators") {
    if (index >= 0) {
      columns.indicators.filter(isOscillator).forEach((ind) => {
        if (!(visibility.indicators[ind.id] ?? true)) return;
        const value = ind.value[index];
        if (Number.isFinite(value)) rows.push({ label: ind.label, value: num(value) });
      });
    }
  } else {
    const point = points.find((p) => p.time === nearest) ?? points.reduce<(typeof points)[number] | null>((best, p) => {
      if (!best) return p;
      return Math.abs(p.time - nearest) < Math.abs(best.time - nearest) ? p : best;
    }, null);
    if (point && visibility.portfolio) rows.push({ label: "Portfolio", value: formatUsdc(point.value) });
    const hold = buyHold.reduce<SeriesPoint | null>((best, p) => {
      if (!best) return p;
      return Math.abs(p.time - nearest) < Math.abs(best.time - nearest) ? p : best;
    }, null);
    if (hold && visibility.buyHold && Math.abs(hold.time - nearest) <= BAR_SEC) {
      rows.push({ label: "Buy & hold", value: formatUsdc(hold.value) });
    }
  }
  const marker = markers.find((m) => Math.abs(m.time - nearest) <= BAR_SEC / 2);
  return {
    time: nearest,
    rows,
    marker: marker ? (marker.side === "buy" ? "Buy" : "Sell") : undefined,
  };
}

export function TimeChart({
  id,
  kind,
  columns,
  points,
  buyHold,
  indicators,
  markers,
  visibility,
  height,
  fromSec,
  toSec,
  timeBand,
  onTimeBand,
  onTimeZoom,
  onReady,
  onRange,
  onReset,
  onMarkerSelect,
  onPainted,
}: {
  id: string;
  kind: ChartKind;
  columns: ChartColumns;
  points: PortfolioPoint[];
  buyHold: SeriesPoint[];
  indicators: ChartIndicatorMeta[];
  markers: TradeMarker[];
  visibility: LayerVisibility;
  height: number;
  fromSec: number;
  toSec: number;
  timeBand: TimeBand | null;
  onTimeBand: (band: TimeBand | null) => void;
  onTimeZoom: (from: number, to: number) => void;
  onReady: (id: string, chart: ChartHandle | null) => void;
  onRange: (id: string, from: number, to: number) => void;
  onReset: () => void;
  onMarkerSelect?: (time: number) => void;
  /** Fired once, after the first draw. */
  onPainted?: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const chartRef = useRef<ECharts | null>(null);
  const windowRef = useRef<TimeBand>({ from: fromSec, to: toSec });
  const [valueBand, setValueBand] = useState<ValueBand | null>(null);
  const [hover, setHover] = useState<Hover | null>(null);
  const propsRef = useRef({ columns, points, buyHold, indicators, markers, visibility, height, fromSec, toSec, timeBand, valueBand, onTimeBand, onTimeZoom, onRange, onReset, onMarkerSelect, onPainted, kind });
  propsRef.current = { columns, points, buyHold, indicators, markers, visibility, height, fromSec, toSec, timeBand, valueBand, onTimeBand, onTimeZoom, onRange, onReset, onMarkerSelect, onPainted, kind };

  const bounds = validBounds(columns, points, fromSec, toSec);
  if (windowRef.current.from < bounds.from || windowRef.current.to > bounds.to) {
    windowRef.current = { from: bounds.from, to: bounds.to };
  }

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    const chart = init(node, undefined, { renderer: "canvas" });
    chartRef.current = chart;
    let applying = false;

    const apply = () => {
      const p = propsRef.current;
      const bound = validBounds(p.columns, p.points, p.fromSec, p.toSec);
      const win = clampWindow(windowRef.current.from, windowRef.current.to, bound.from, bound.to);
      windowRef.current = win;
      applying = true;
      chart.setOption(
        buildOption({
          kind: p.kind,
          theme: readChartTheme(),
          font: chartFontFamily(),
          columns: p.columns,
          points: p.points,
          buyHold: p.buyHold,
          indicators: p.indicators,
          markers: p.markers,
          visibility: p.visibility,
          height: p.height,
          fromSec: bound.from,
          toSec: bound.to,
          window: win,
          timeBand: p.timeBand,
          valueBand: p.valueBand,
        }),
        { notMerge: true },
      );
      applying = false;
      chart.resize({ width: node.clientWidth, height: p.height });
    };

    const setWindow = (from: number, to: number) => {
      const bound = validBounds(propsRef.current.columns, propsRef.current.points, propsRef.current.fromSec, propsRef.current.toSec);
      const next = clampWindow(from, to, bound.from, bound.to);
      if (sameWindow(next, windowRef.current)) return;
      windowRef.current = next;
      apply();
    };

    chart.on("datazoom", () => {
      if (applying) return;
      const bound = validBounds(propsRef.current.columns, propsRef.current.points, propsRef.current.fromSec, propsRef.current.toSec);
      const next = readWindow(chart, bound.from, bound.to);
      if (sameWindow(next, windowRef.current)) return;
      windowRef.current = next;
      propsRef.current.onRange(id, next.from, next.to);
      apply();
    });

    chart.on("click", (params) => {
      const hit = params as { componentType?: string; name?: string };
      if (hit.componentType !== "markPoint" || !hit.name) return;
      const time = Number(hit.name);
      if (Number.isFinite(time)) propsRef.current.onMarkerSelect?.(time);
    });

    const gridCount = () => (propsRef.current.kind === "oscillators"
      ? Math.max(1, propsRef.current.indicators.filter(isOscillator).length)
      : 1);

    let drag: { x: number; y: number; kind: "time" | "value" | null; axis: number; time: TimeBand | null } | null = null;
    const onDown = (event: PointerEvent) => {
      if (event.button !== 0) return;
      drag = { x: event.offsetX, y: event.offsetY, kind: null, axis: gridIndexAt(event.offsetY, gridCount(), propsRef.current.height), time: null };
      try {
        node.setPointerCapture(event.pointerId);
      } catch {
        // A pointer id can already be gone. The drag still tracks pointermove.
      }
    };
    const onMove = (event: PointerEvent) => {
      if (!drag) {
        const axis = gridIndexAt(event.offsetY, gridCount(), propsRef.current.height);
        const hit = pixelOnAxis(chart, event.offsetX, event.offsetY, axis);
        const time = hit == null ? null : hit[0] / 1000;
        const p = propsRef.current;
        setHover(time == null ? null : hoverAt(p.kind, time, p.columns, p.points, p.buyHold, p.indicators, p.markers, p.visibility));
        return;
      }
      const dx = event.offsetX - drag.x;
      const dy = event.offsetY - drag.y;
      if (!drag.kind) {
        if (Math.hypot(dx, dy) < 5) return;
        drag.kind = Math.abs(dx) >= Math.abs(dy) ? "time" : "value";
      }
      if (drag.kind === "time") {
        const a = pixelOnAxis(chart, drag.x, drag.y, drag.axis);
        const b = pixelOnAxis(chart, event.offsetX, event.offsetY, drag.axis);
        if (a == null || b == null) return;
        const band = { from: Math.min(a[0], b[0]) / 1000, to: Math.max(a[0], b[0]) / 1000 };
        drag.time = band;
        propsRef.current.onTimeBand(band);
      } else {
        const a = pixelOnAxis(chart, drag.x, drag.y, drag.axis);
        const b = pixelOnAxis(chart, event.offsetX, event.offsetY, drag.axis);
        if (a == null || b == null) return;
        setValueBand({ axis: drag.axis, min: Math.min(a[1], b[1]), max: Math.max(a[1], b[1]) });
      }
    };
    const onUp = () => {
      if (!drag) return;
      const finished = drag;
      drag = null;
      if (finished.kind == null) {
        propsRef.current.onTimeBand(null);
        setValueBand(null);
        return;
      }
      if (finished.kind === "time" && finished.time && finished.time.to > finished.time.from) {
        propsRef.current.onTimeZoom(finished.time.from, finished.time.to);
      }
    };
    const onLeave = () => { if (!drag) setHover(null); };
    const onDblClick = () => propsRef.current.onReset();
    node.addEventListener("pointerdown", onDown);
    node.addEventListener("pointermove", onMove);
    node.addEventListener("pointerup", onUp);
    node.addEventListener("pointercancel", onUp);
    node.addEventListener("pointerleave", onLeave);
    node.addEventListener("dblclick", onDblClick);

    const resize = new ResizeObserver(() => {
      chart.resize({ width: node.clientWidth, height: propsRef.current.height });
    });
    resize.observe(node);
    const themeWatch = new MutationObserver(() => apply());
    themeWatch.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

    onReady(id, { setWindow });
    apply();
    propsRef.current.onPainted?.();

    return () => {
      node.removeEventListener("pointerdown", onDown);
      node.removeEventListener("pointermove", onMove);
      node.removeEventListener("pointerup", onUp);
      node.removeEventListener("pointercancel", onUp);
      node.removeEventListener("pointerleave", onLeave);
      node.removeEventListener("dblclick", onDblClick);
      resize.disconnect();
      themeWatch.disconnect();
      onReady(id, null);
      chart.dispose();
      chartRef.current = null;
    };
    // Recreate only when this pane's identity changes. Data updates go through apply().
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, kind]);

  useEffect(() => {
    const chart = chartRef.current;
    const node = ref.current;
    if (!chart || !node) return;
    const p = propsRef.current;
    const bound = validBounds(p.columns, p.points, p.fromSec, p.toSec);
    const win = clampWindow(windowRef.current.from, windowRef.current.to, bound.from, bound.to);
    windowRef.current = win;
    chart.setOption(
      buildOption({
        kind,
        theme: readChartTheme(),
        font: chartFontFamily(),
        columns,
        points,
        buyHold,
        indicators,
        markers,
        visibility,
        height,
        fromSec: bound.from,
        toSec: bound.to,
        window: win,
        timeBand,
        valueBand,
      }),
      { notMerge: true },
    );
    chart.resize({ width: node.clientWidth, height });
  }, [columns, points, buyHold, indicators, markers, visibility, height, fromSec, toSec, timeBand, valueBand, kind]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented || !valueBand) return;
      event.preventDefault();
      setValueBand(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [valueBand]);

  const hasContent = kind === "portfolio" ? points.length > 0 || columns.timeSec.length > 0 : columns.timeSec.length > 0;

  return (
    <div className="relative">
      <div ref={ref} data-chart-plot className="w-full" style={{ height, touchAction: "none" }} />
      {!hasContent && <p className="py-6 text-center text-xs text-muted-foreground">No data for this chart.</p>}
      {hover && (
        <div className="pointer-events-none absolute left-2 top-2 z-10 space-y-0.5 rounded-md border bg-background/95 px-2 py-1.5 text-xs shadow-sm">
          <p className="font-mono text-muted-foreground">{formatUtcChartTime(hover.time)}</p>
          {hover.rows.map((row) => (
            <p key={row.label} className="text-muted-foreground">
              {row.label} {row.value}
            </p>
          ))}
          {hover.marker && <p className="font-medium">{hover.marker} · click to open the decision</p>}
        </div>
      )}
    </div>
  );
}

function validBounds(columns: ChartColumns, points: PortfolioPoint[], fromSec: number, toSec: number): TimeBand {
  if (Number.isFinite(fromSec) && Number.isFinite(toSec) && toSec > fromSec) return { from: fromSec, to: toSec };
  const times = [...columns.timeSec, ...points.map((p) => p.time)];
  if (times.length === 0) return { from: 0, to: 1 };
  return { from: Math.min(...times), to: Math.max(...times) };
}
