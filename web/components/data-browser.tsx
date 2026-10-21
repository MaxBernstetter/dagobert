"use client";

import { useState } from "react";
import { KpiCharts } from "@/components/kpi-charts";
import { SegmentChart } from "@/components/segment-chart";
import type { PositionView } from "@/lib/read";

const POSITIONS = [
  { symbol: "BTCUSDT", label: "BTC" },
  { symbol: "ETHUSDT", label: "ETH" },
];

export function DataBrowser({ algos }: { algos: string[] }) {
  const [symbol, setSymbol] = useState("");
  const [algo, setAlgo] = useState(algos[algos.length - 1] ?? "v1");
  const [view, setView] = useState<PositionView | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function load(nextSymbol: string, nextAlgo: string) {
    setSymbol(nextSymbol);
    setAlgo(nextAlgo);
    setLoading(true);
    setError("");
    const res = await fetch(`/api/data?symbol=${nextSymbol}&algo=${encodeURIComponent(nextAlgo)}`);
    setLoading(false);
    if (!res.ok) {
      setView(null);
      setError(await res.text());
      return;
    }
    setView(await res.json());
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end gap-3">
        {POSITIONS.map((position) => (
          <button
            key={position.symbol}
            type="button"
            className={`rounded-md border px-3 py-1 text-sm ${
              symbol === position.symbol ? "bg-foreground text-background" : "bg-background"
            }`}
            onClick={() => load(position.symbol, algo)}
          >
            {position.label}
          </button>
        ))}
        <label className="space-y-1 text-sm">
          <span className="text-muted-foreground">Algorithm</span>
          <select
            value={algo}
            className="block rounded-md border bg-background px-2 py-1"
            onChange={(event) => {
              const next = event.target.value;
              setAlgo(next);
              if (symbol) load(symbol, next);
            }}
          >
            {algos.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
        </label>
      </div>
      {!symbol && <p className="text-sm text-muted-foreground">Choose a position to load its range and KPIs.</p>}
      {loading && <p className="text-sm text-muted-foreground">Loading {symbol}…</p>}
      {error && <p className="text-sm text-destructive">{error}</p>}
      {view && !loading && (
        <section className="space-y-6">
          <div>
            <h2 className="text-xl font-medium">{view.label}</h2>
            <p className="text-sm text-muted-foreground">
              Source: Binance public klines ({view.provider}). Orders use Bybit {view.label === "BTC" ? "BTCUSDC" : "ETHUSDC"}. Algorithm {view.algo}. The chart is not the Bybit print.
            </p>
          </div>
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-muted-foreground">Data range</h3>
            <SegmentChart start={view.start} cells={view.cells} />
          </div>
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-muted-foreground">KPIs</h3>
            <KpiCharts bars={view.bars} />
          </div>
        </section>
      )}
    </div>
  );
}
