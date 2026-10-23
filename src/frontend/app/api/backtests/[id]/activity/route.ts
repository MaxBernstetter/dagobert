import { NextResponse } from "next/server";

import { backtestWorkspaceDir, getBacktest, playbookIdForKpi } from "@/lib/backtest-catalog";
import { readActivity, readJournals } from "@/lib/read";

export const dynamic = "force-dynamic";

type Params = { params: Promise<{ id: string }> };

export async function GET(_request: Request, { params }: Params) {
  const { id } = await params;
  const rec = getBacktest(id);
  if (!rec) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  const ws = backtestWorkspaceDir(rec);
  const playbookId = playbookIdForKpi(rec.settings.playbook);
  return NextResponse.json({
    rows: readActivity(ws),
    journals: readJournals(ws),
    playbookTitles: { [playbookId]: rec.name },
    backtestId: id,
    playbook: playbookId,
    symbol: rec.settings.symbol,
  });
}
