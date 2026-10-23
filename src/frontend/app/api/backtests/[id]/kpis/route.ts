import { NextResponse } from "next/server";

import {
  backtestWorkspaceDir,
  getBacktest,
  playbookIdForKpi,
} from "@/lib/backtest-catalog";
import { backtestStartCash } from "@/lib/backtest-shared";
import { computeStrategyKPIs } from "@/lib/strategy-kpi";

export const dynamic = "force-dynamic";

type Params = { params: Promise<{ id: string }> };

export async function GET(request: Request, { params }: Params) {
  const { id } = await params;
  const rec = getBacktest(id);
  if (!rec) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  const url = new URL(request.url);
  const fromMs = Number(url.searchParams.get("fromMs") ?? rec.settings.fromMs);
  const toMs = Number(url.searchParams.get("toMs") ?? rec.settings.toMs);
  const playbook = playbookIdForKpi(rec.settings.playbook);
  const ws = backtestWorkspaceDir(rec);
  try {
    const set = computeStrategyKPIs(playbook, rec.settings.symbol, fromMs, toMs, {
      workspaceDir: ws,
      backtestId: id,
      startCash: backtestStartCash(rec.settings),
    });
    return NextResponse.json(set);
  } catch (err) {
    const message = err instanceof Error ? err.message : "kpi compute failed";
    return NextResponse.json({ error: message }, { status: 400 });
  }
}
