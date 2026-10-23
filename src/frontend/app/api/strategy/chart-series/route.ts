import { NextResponse } from "next/server";
import { chartIndicatorsForPlaybook } from "@/lib/playbook-indicators";
import { readPlaybookChartBars } from "@/lib/read";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const playbook = url.searchParams.get("playbook")?.trim() ?? "";
  const symbol = url.searchParams.get("symbol")?.trim() ?? "";
  const algo = url.searchParams.get("algo")?.trim() || "v1";
  const fromSec = Number(url.searchParams.get("from"));
  const toSec = Number(url.searchParams.get("to"));

  if (!playbook || !symbol || !Number.isFinite(fromSec) || !Number.isFinite(toSec)) {
    return NextResponse.json({ error: "playbook, symbol, from, to required" }, { status: 400 });
  }

  const indicators = chartIndicatorsForPlaybook(playbook).map(({ id, label, color }) => ({
    id,
    label,
    color,
  }));

  const bars = readPlaybookChartBars(symbol, algo, fromSec, toSec, chartIndicatorsForPlaybook(playbook));

  return NextResponse.json({ bars, indicators });
}
