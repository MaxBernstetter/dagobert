import { NextResponse } from "next/server";

import { getBacktest, markBacktestRunning } from "@/lib/backtest-catalog";
import { triggerBacktestRun } from "@/lib/backtest-run";

export const dynamic = "force-dynamic";

type Params = { params: Promise<{ id: string }> };

export async function POST(_request: Request, { params }: Params) {
  const { id } = await params;
  const rec = getBacktest(id);
  if (!rec) {
    return NextResponse.json({ error: "backtest not found in catalog" }, { status: 404 });
  }
  const staleStarting =
    rec.status === "running" &&
    rec.progressStageLabel === "Starting" &&
    rec.startedAt != null &&
    Date.now() - rec.startedAt > 45_000;
  if (rec.status === "running" && !staleStarting) {
    return NextResponse.json({ error: "already running" }, { status: 409 });
  }
  const running = markBacktestRunning(id) ?? rec;
  const result = await triggerBacktestRun(id);
  if (!result.ok) {
    return NextResponse.json({ error: result.error }, { status: result.status });
  }
  return NextResponse.json(running, { status: 202 });
}
