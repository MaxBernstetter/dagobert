import { NextResponse } from "next/server";

import { resolveBacktestSettings } from "@/lib/backtest-catalog";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const symbol = url.searchParams.get("symbol")?.trim();
  const settings = resolveBacktestSettings(symbol ? { symbol } : undefined);
  return NextResponse.json(settings);
}
