
import { useEffect, useRef, useState } from "react";
import { init, type ECharts } from "@/components/echarts-setup";
import {
  AXIS_TITLE_MIN_PX,
  chartFontFamily,
  formatUtcTick,
  readChartTheme,
} from "@/components/chart-theme";
import type { PeriodReturn } from "@/lib/experiment-shared";
import { cn } from "@/lib/utils";

function periodToMs(period: string): number | null {
  const match = /^(\d{4})-(\d{2})$/.exec(period);
  if (!match) return null;
  const year = Number(match[1]);
  const month = Number(match[2]);
  if (!Number.isFinite(year) || !Number.isFinite(month)) return null;
  return Date.UTC(year, month - 1, 1);
}

function formatMonth(ms: number): string {
  const d = new Date(ms);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

export function MonthlyReturnChart({
  series,
  className,
  title = "Monthly returns",
  height = 220,
  embedded = false,
  fill = false,
  onPainted,
}: {
  series: PeriodReturn[];
  className?: string;
  title?: string;
  height?: number;
  /** Title is provided by a surrounding section. */
  embedded?: boolean;
  /** Grow with the section instead of a fixed height. */
  fill?: boolean;
  /** Fired once, after the first draw. */
  onPainted?: () => void;
}) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ECharts | null>(null);
  const [hover, setHover] = useState<{ time: number; value: number } | null>(null);
  const seriesRef = useRef(series);
  seriesRef.current = series;
  const heightRef = useRef(height);
  heightRef.current = height;
  const onPaintedRef = useRef(onPainted);
  onPaintedRef.current = onPainted;

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const chart = init(host, undefined, { renderer: "canvas" });
    chartRef.current = chart;

    const draw = () => {
      const theme = readChartTheme();
      const rows = seriesRef.current
        .map((row) => {
          const time = periodToMs(row.period);
          if (time == null || !Number.isFinite(row.return)) return null;
          return [time, row.return * 100] as [number, number];
        })
        .filter((row): row is [number, number] => row != null)
        .sort((a, b) => a[0] - b[0]);
      const spanSec = rows.length > 1 ? (rows[rows.length - 1][0] - rows[0][0]) / 1000 : 86400 * 365;
      const showTitle = heightRef.current >= AXIS_TITLE_MIN_PX;
      const from = rows[0]?.[0];
      const to = rows[rows.length - 1]?.[0];
      chart.setOption({
        animation: false,
        backgroundColor: "transparent",
        textStyle: { color: theme.text, fontFamily: chartFontFamily() },
        grid: { left: showTitle ? 72 : 52, right: 16, top: 12, bottom: showTitle ? 36 : 24 },
        xAxis: {
          type: "time",
          min: from,
          max: to,
          name: showTitle ? "UTC" : "",
          nameLocation: "middle",
          nameGap: 24,
          nameTextStyle: { color: theme.text, fontSize: 11 },
          axisLine: { lineStyle: { color: theme.axisLine } },
          axisLabel: {
            color: theme.text,
            fontSize: 11,
            hideOverlap: true,
            formatter: (value: number) => formatUtcTick(value, spanSec),
          },
          splitLine: { show: true, lineStyle: { color: theme.grid } },
        },
        yAxis: {
          type: "value",
          name: showTitle ? "Return (%)" : "",
          nameLocation: "middle",
          nameRotate: 90,
          nameGap: 48,
          nameTextStyle: { color: theme.text, fontSize: 11 },
          scale: true,
          axisLine: { show: false },
          axisLabel: { color: theme.text, fontSize: 11, formatter: (value: number) => `${formatCompact(value)}%` },
          splitLine: { show: true, lineStyle: { color: theme.grid } },
        },
        dataZoom: [{
          type: "inside",
          filterMode: "weakFilter",
          zoomOnMouseWheel: true,
          moveOnMouseMove: false,
          moveOnMouseWheel: false,
          throttle: 40,
        }],
        series: [{
          type: "bar",
          data: rows,
          itemStyle: {
            color: (params: { value: number[] }) => ((params.value[1] ?? 0) >= 0 ? theme.positive : theme.negative),
          },
          markLine: {
            silent: true,
            symbol: "none",
            label: { show: false },
            lineStyle: { color: theme.hold, type: "dashed" },
            data: [{ yAxis: 0 }],
          },
        }],
      }, { notMerge: true });
      chart.resize({ width: host.clientWidth, height: heightRef.current });
    };

    const onMove = (event: { offsetX: number; offsetY: number }) => {
      const raw = chart.convertFromPixel({ xAxisIndex: 0, yAxisIndex: 0 }, [event.offsetX, event.offsetY]);
      const ms = Number(Array.isArray(raw) ? raw[0] : raw);
      if (!Number.isFinite(ms)) {
        setHover(null);
        return;
      }
      const rows = seriesRef.current
        .map((row) => {
          const time = periodToMs(row.period);
          return time == null ? null : { time, value: row.return * 100 };
        })
        .filter((row): row is { time: number; value: number } => row != null);
      let best: { time: number; value: number } | null = null;
      for (const row of rows) {
        if (!best || Math.abs(row.time - ms) < Math.abs(best.time - ms)) best = row;
      }
      setHover(best && Math.abs(best.time - ms) < 20 * 86400 * 1000 ? best : null);
    };
    chart.getZr().on("mousemove", onMove);
    chart.getZr().on("globalout", () => setHover(null));

    const resize = new ResizeObserver(() => {
      chart.resize({ width: host.clientWidth, height: heightRef.current });
    });
    resize.observe(host);
    const themeWatch = new MutationObserver(draw);
    themeWatch.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    draw();
    onPaintedRef.current?.();

    return () => {
      resize.disconnect();
      themeWatch.disconnect();
      chart.dispose();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    const host = hostRef.current;
    const chart = chartRef.current;
    if (!host || !chart) return;
    chart.resize({ width: host.clientWidth, height });
    const theme = readChartTheme();
    const showTitle = height >= AXIS_TITLE_MIN_PX;
    chart.setOption({
      grid: { left: showTitle ? 72 : 52, right: 16, top: 12, bottom: showTitle ? 36 : 24 },
      xAxis: { name: showTitle ? "UTC" : "" },
      yAxis: { name: showTitle ? "Return (%)" : "" },
      textStyle: { color: theme.text },
    });
  }, [height]);

  useEffect(() => {
    seriesRef.current = series;
    const chart = chartRef.current;
    if (!chart) return;
    const rows = series
      .map((row) => {
        const time = periodToMs(row.period);
        if (time == null || !Number.isFinite(row.return)) return null;
        return [time, row.return * 100] as [number, number];
      })
      .filter((row): row is [number, number] => row != null)
      .sort((a, b) => a[0] - b[0]);
    const theme = readChartTheme();
    chart.setOption({
      xAxis: { min: rows[0]?.[0], max: rows[rows.length - 1]?.[0] },
      series: [{
        type: "bar",
        data: rows,
        itemStyle: {
          color: (params: { value: number[] }) => ((params.value[1] ?? 0) >= 0 ? theme.positive : theme.negative),
        },
        markLine: {
          silent: true,
          symbol: "none",
          label: { show: false },
          lineStyle: { color: theme.hold, type: "dashed" },
          data: [{ yAxis: 0 }],
        },
      }],
    });
  }, [series]);

  if (!series.length) return null;

  return (
    <div className={cn("space-y-2", fill && "xl:flex xl:min-h-0 xl:flex-1 xl:flex-col", className)}>
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
        {!embedded && <p className="text-xs font-medium text-muted-foreground">{title}</p>}
        <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
          {hover && (
            <span className="font-mono text-foreground">
              {formatMonth(hover.time)}: {hover.value.toFixed(2)}%
            </span>
          )}
          <span className="inline-flex items-center gap-1.5">
            <span className="inline-block size-2.5 rounded-sm" style={{ backgroundColor: readChartTheme().positive }} aria-hidden />
            Positive month
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="inline-block size-2.5 rounded-sm" style={{ backgroundColor: readChartTheme().negative }} aria-hidden />
            Negative month
          </span>
        </div>
      </div>
      <div ref={hostRef} data-chart-plot className="w-full" style={{ height, touchAction: "none" }} />
    </div>
  );
}

function formatCompact(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 10) return value.toLocaleString(undefined, { maximumFractionDigits: 0 });
  return value.toLocaleString(undefined, { maximumFractionDigits: 1 });
}
