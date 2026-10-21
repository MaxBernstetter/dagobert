"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import type { LiveState } from "@/lib/read";

const STAGE_LABEL: Record<string, string> = {
  ingest: "Ingesting",
  process: "Processing",
  indicators: "Recalculating",
  strategy: "Deciding",
  execution: "Filling",
};

function formatMs(ms: number | null): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

function ago(iso: string | null): string {
  if (!iso) return "—";
  const delta = Date.now() - new Date(iso).getTime();
  if (!Number.isFinite(delta) || delta < 0) return iso;
  const sec = Math.round(delta / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.round(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  return `${hr}h ago`;
}

function nextClose(now: number): string {
  const step = 3 * 60 * 60 * 1000;
  const next = Math.ceil(now / step) * step;
  return formatMs(next === now ? now + step : next);
}

export function LiveStatus({ live }: { live: LiveState }) {
  const router = useRouter();
  useEffect(() => {
    const id = setInterval(() => router.refresh(), 4000);
    return () => clearInterval(id);
  }, [router]);

  const running = live.stage !== "";
  const label = STAGE_LABEL[live.stage] ?? "Idle";

  return (
    <div className="flex items-center gap-3">
      <span
        className={`inline-block size-2.5 rounded-full ${
          running ? "animate-pulse bg-emerald-500" : "bg-muted-foreground/40"
        }`}
      />
      <h2 className="text-lg font-medium">{running ? label : "Idle"}</h2>
      <p className="text-sm text-muted-foreground">
        {running
          ? `${live.symbol} · candle ${formatMs(live.candleClose)} · started ${ago(live.since)}`
          : `Next close ${nextClose(Date.now())}${
              live.lastFinished ? ` · last ${live.lastStage} ${ago(live.lastFinished)}` : ""
            }`}
      </p>
    </div>
  );
}
