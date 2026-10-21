"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PositionSelect } from "@/components/position-select";

export type BackfillStatus = {
  kind: string;
  from: number;
  to: number;
  symbol: string;
  running: boolean;
  error: string;
  finished: string;
};

function dayStart(value: string): number {
  return Date.parse(`${value}T00:00:00Z`);
}

export function BackfillForm({ initial }: { initial: BackfillStatus }) {
  const [from, setFrom] = useState("2022-01-01");
  const [to, setTo] = useState("");
  const [status, setStatus] = useState(initial);
  const [message, setMessage] = useState("");
  const [symbols, setSymbols] = useState<string[]>(["BTCUSDT", "ETHUSDT"]);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    const id = setInterval(async () => {
      const res = await fetch("/api/backfill");
      if (res.ok) setStatus(await res.json());
    }, 4000);
    return () => clearInterval(id);
  }, []);

  async function start(kind: "ingest" | "indicators") {
    setMessage("");
    const fromMs = dayStart(from);
    const toMs = to ? Math.min(Date.now(), dayStart(to) + 86_400_000) : Date.now();
    if (!Number.isFinite(fromMs) || !Number.isFinite(toMs) || fromMs >= toMs) {
      setMessage("From must be a UTC date before to.");
      return;
    }
    if (symbols.length === 0) {
      setMessage("Choose at least one position.");
      return;
    }
    setPending(true);
    const res = await fetch("/api/backfill", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ kind, from: fromMs, to: toMs, symbols }),
    });
    setPending(false);
    if (!res.ok) {
      setMessage(await res.text());
      return;
    }
    setStatus(await res.json());
  }

  const label = status.running
    ? `${status.kind === "indicators" ? "Recalculating indicators" : "Backfilling"} ${status.symbol}`
    : status.finished
      ? `Last ${status.kind} finished ${status.finished}${status.error ? ` · ${status.error}` : ""}`
      : "No backfill has run in this stack yet.";

  return (
    <Card>
      <CardHeader>
        <CardTitle>Backfill</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          Dates are UTC. An empty end date means now. Data backfill stores hourly candles.
          Recalculate indicators rewrites KPI rows for every closed 3-hour bar in the range.
          Neither step places an order.
        </p>
        <div className="flex flex-wrap items-end gap-3">
          <label className="space-y-1 text-sm">
            <span className="text-muted-foreground">From</span>
            <input
              type="date"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
              className="block rounded-md border bg-background px-2 py-1"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="text-muted-foreground">To</span>
            <input
              type="date"
              value={to}
              onChange={(event) => setTo(event.target.value)}
              className="block rounded-md border bg-background px-2 py-1"
            />
          </label>
          <PositionSelect value={symbols} onChange={setSymbols} />
          <Button disabled={pending || status.running} onClick={() => start("ingest")}>
            Backfill data
          </Button>
          <Button
            variant="outline"
            disabled={pending || status.running}
            onClick={() => start("indicators")}
          >
            Recalculate indicators
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">{message || label}</p>
      </CardContent>
    </Card>
  );
}
