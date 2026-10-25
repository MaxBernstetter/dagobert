import { useEffect, useMemo, useState } from "react";
import { chartSeries, type ChartBar, type ChartIndicatorMeta } from "./api";

type Props = {
  experimentId: string;
  instanceId: string;
  defaultFromSec: number;
  defaultToSec: number;
};

function draw(canvas: HTMLCanvasElement, bars: ChartBar[]) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const dpr = window.devicePixelRatio || 1;
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  canvas.width = Math.floor(width * dpr);
  canvas.height = Math.floor(height * dpr);
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.fillStyle = "#1c1915";
  ctx.fillRect(0, 0, width, height);
  if (bars.length === 0) return;
  let lo = Infinity;
  let hi = -Infinity;
  for (const b of bars) {
    lo = Math.min(lo, b.low);
    hi = Math.max(hi, b.high);
  }
  if (!Number.isFinite(lo) || !Number.isFinite(hi) || hi === lo) {
    hi = lo + 1;
  }
  const pad = 12;
  const slot = width / bars.length;
  const y = (price: number) => pad + ((hi - price) / (hi - lo)) * (height - pad * 2);
  bars.forEach((b, i) => {
    const x = i * slot + slot / 2;
    const up = b.close >= b.open;
    ctx.strokeStyle = up ? "#3ddc97" : "#e15b64";
    ctx.fillStyle = ctx.strokeStyle;
    ctx.beginPath();
    ctx.moveTo(x, y(b.high));
    ctx.lineTo(x, y(b.low));
    ctx.stroke();
    const top = y(Math.max(b.open, b.close));
    const bot = y(Math.min(b.open, b.close));
    const h = Math.max(1, bot - top);
    ctx.fillRect(x - Math.max(1, slot * 0.3), top, Math.max(2, slot * 0.6), h);
  });
}

export function ChartPanel({ experimentId, instanceId, defaultFromSec, defaultToSec }: Props) {
  const [fromSec, setFromSec] = useState(defaultFromSec);
  const [toSec, setToSec] = useState(defaultToSec);
  const [bars, setBars] = useState<ChartBar[]>([]);
  const [indicators, setIndicators] = useState<ChartIndicatorMeta[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setFromSec(defaultFromSec);
    setToSec(defaultToSec);
  }, [defaultFromSec, defaultToSec, instanceId]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    chartSeries(experimentId, instanceId, fromSec, toSec)
      .then((data) => {
        if (cancelled) return;
        setBars(data.bars ?? []);
        setIndicators(data.indicators ?? []);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setBars([]);
        setIndicators([]);
        setError(err instanceof Error ? err.message : "Chart request failed");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [experimentId, instanceId, fromSec, toSec]);

  const canvasRef = useMemo(() => ({ current: null as HTMLCanvasElement | null }), []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    draw(canvas, bars);
  }, [bars, canvasRef]);

  return (
    <section className="panel">
      <h2>Chart</h2>
      <p className="muted">
        GET /api/v1/experiments/{experimentId}/instances/{instanceId}/chart-series
      </p>
      <div className="row" style={{ marginBottom: "0.75rem" }}>
        <label>
          From (unix seconds)
          <input
            type="number"
            value={fromSec}
            onChange={(e) => setFromSec(Number(e.target.value))}
          />
        </label>
        <label>
          To (unix seconds)
          <input type="number" value={toSec} onChange={(e) => setToSec(Number(e.target.value))} />
        </label>
        {loading && <span className="muted">Loading…</span>}
      </div>
      {error && <p className="error">{error}</p>}
      <canvas
        className="chart"
        ref={(node) => {
          canvasRef.current = node;
          if (node) draw(node, bars);
        }}
      />
      <p className="muted">
        {bars.length} bars
        {indicators.length > 0 ? ` · ${indicators.map((i) => i.label || i.id).join(", ")}` : ""}
      </p>
    </section>
  );
}
