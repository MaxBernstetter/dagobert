import { NextResponse } from "next/server";
import { computeStrategyKPIs } from "@/lib/strategy-kpi";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const playbook = url.searchParams.get("playbook")?.trim() ?? "";
  const symbol = url.searchParams.get("symbol")?.trim() ?? "";
  const fromMs = Number(url.searchParams.get("fromMs") ?? url.searchParams.get("from") ?? 0);
  const toMs = Number(url.searchParams.get("toMs") ?? url.searchParams.get("to") ?? 0);

  if (!playbook || !symbol) {
    return NextResponse.json({ error: "playbook and symbol required" }, { status: 400 });
  }

  try {
    const set = computeStrategyKPIs(playbook, symbol, fromMs, toMs);
    return NextResponse.json(set);
  } catch (err) {
    const message = err instanceof Error ? err.message : "kpi compute failed";
    return NextResponse.json({ error: message }, { status: 400 });
  }
}
