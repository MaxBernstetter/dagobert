"use client";

import { useEffect, useRef } from "react";
import { ColorType, createChart, LineSeries, CandlestickSeries } from "lightweight-charts";
import type { PositionView } from "@/lib/read";

export function KpiCharts({ bars }: { bars: PositionView["bars"] }) {
  const priceRef = useRef<HTMLDivElement>(null);
  const rsiRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const priceNode = priceRef.current;
    const rsiNode = rsiRef.current;
    if (!priceNode || !rsiNode || bars.length === 0) return;

    const dark = document.documentElement.classList.contains("dark");
    const textColor = dark ? "#a1a1aa" : "#71717a";
    const gridColor = dark ? "#27272a" : "#e4e4e7";

    const price = createChart(priceNode, {
      height: 280,
      layout: { background: { type: ColorType.Solid, color: "transparent" }, textColor },
      grid: { vertLines: { color: gridColor }, horzLines: { color: gridColor } },
      timeScale: { timeVisible: true },
    });
    const candles = price.addSeries(CandlestickSeries, {
      upColor: "#22c55e",
      downColor: "#ef4444",
      wickUpColor: "#22c55e",
      wickDownColor: "#ef4444",
      borderVisible: false,
    });
    candles.setData(bars.map((bar) => ({ time: bar.time as never, open: bar.open, high: bar.high, low: bar.low, close: bar.close })));
    const sma3 = price.addSeries(LineSeries, { color: "#38bdf8", lineWidth: 2, priceLineVisible: false });
    sma3.setData(bars.filter((bar) => bar.sma3 != null).map((bar) => ({ time: bar.time as never, value: bar.sma3 as number })));
    const sma6 = price.addSeries(LineSeries, { color: "#f59e0b", lineWidth: 2, priceLineVisible: false });
    sma6.setData(bars.filter((bar) => bar.sma6 != null).map((bar) => ({ time: bar.time as never, value: bar.sma6 as number })));
    price.timeScale().fitContent();

    const rsi = createChart(rsiNode, {
      height: 140,
      layout: { background: { type: ColorType.Solid, color: "transparent" }, textColor },
      grid: { vertLines: { color: gridColor }, horzLines: { color: gridColor } },
      timeScale: { timeVisible: true },
    });
    const rsiLine = rsi.addSeries(LineSeries, { color: "#c084fc", lineWidth: 2, priceLineVisible: false });
    rsiLine.setData(bars.filter((bar) => bar.rsi != null).map((bar) => ({ time: bar.time as never, value: bar.rsi as number })));
    rsi.timeScale().fitContent();

    const onResize = () => {
      price.applyOptions({ width: priceNode.clientWidth });
      rsi.applyOptions({ width: rsiNode.clientWidth });
    };
    onResize();
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      price.remove();
      rsi.remove();
    };
  }, [bars]);

  if (bars.length === 0) {
    return <p className="text-sm text-muted-foreground">No KPI rows for this position.</p>;
  }

  return (
    <div className="space-y-3">
      <div>
        <p className="mb-1 text-sm text-muted-foreground">Price, 3h SMA, 6h SMA</p>
        <div ref={priceRef} />
      </div>
      <div>
        <p className="mb-1 text-sm text-muted-foreground">RSI</p>
        <div ref={rsiRef} />
      </div>
    </div>
  );
}
