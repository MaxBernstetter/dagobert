import { NextResponse } from "next/server";

import { getBacktest, playbookIdForKpi } from "@/lib/backtest-catalog";
import { chartIndicatorsForPlaybook } from "@/lib/playbook-indicators";
import { readPlaybookChartBars } from "@/lib/read";

export const dynamic = "force-dynamic";

type Params = { params: Promise<{ id: string }> };

export async function GET(request: Request, { params }: Params) {
  const { id } = await params;
  const rec = getBacktest(id);
  if (!rec) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  const url = new URL(request.url);
  const fromSec = Number(url.searchParams.get("from"));
  const toSec = Number(url.searchParams.get("to"));
  const playbook = playbookIdForKpi(rec.settings.playbook);
  const symbol = rec.settings.symbol;
  const algo = rec.settings.algo || "v1";

  if (!Number.isFinite(fromSec) || !Number.isFinite(toSec)) {
    return NextResponse.json({ error: "from and to (unix seconds) required" }, { status: 400 });
  }

  const indicators = chartIndicatorsForPlaybook(playbook).map(({ id: indId, label, color }) => ({
    id: indId,
    label,
    color,
  }));
  const bars = readPlaybookChartBars(
    symbol,
    algo,
    fromSec,
    toSec,
    chartIndicatorsForPlaybook(playbook),
    id,
  );
  return NextResponse.json({ bars, indicators });
}
