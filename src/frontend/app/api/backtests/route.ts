import { NextResponse } from "next/server";

import { createBacktest, listBacktests, type BacktestSettings } from "@/lib/backtest-catalog";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json({ backtests: listBacktests() });
}

export async function POST(request: Request) {
  let body: { name?: string; description?: string; settings?: Partial<BacktestSettings> };
  try {
    body = (await request.json()) as typeof body;
  } catch {
    return NextResponse.json({ error: "invalid json" }, { status: 400 });
  }
  const name = body.name?.trim() || `Backtest ${new Date().toISOString().slice(0, 10)}`;
  const rec = createBacktest(name, body.description ?? "", body.settings);
  return NextResponse.json(rec, { status: 201 });
}
