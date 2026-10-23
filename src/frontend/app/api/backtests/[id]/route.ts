import { NextResponse } from "next/server";

import { getBacktest, updateBacktest, type BacktestSettings } from "@/lib/backtest-catalog";

export const dynamic = "force-dynamic";

type Params = { params: Promise<{ id: string }> };

export async function GET(_request: Request, { params }: Params) {
  const { id } = await params;
  const rec = getBacktest(id);
  if (!rec) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  return NextResponse.json(rec);
}

export async function PATCH(request: Request, { params }: Params) {
  const { id } = await params;
  let body: { name?: string; description?: string; settings?: BacktestSettings };
  try {
    body = (await request.json()) as typeof body;
  } catch {
    return NextResponse.json({ error: "invalid json" }, { status: 400 });
  }
  const existing = getBacktest(id);
  if (!existing) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  const updated = updateBacktest(
    id,
    body.name ?? existing.name,
    body.description ?? existing.description,
    body.settings ?? existing.settings,
  );
  if (!updated) {
    return NextResponse.json({ error: "cannot update while running" }, { status: 409 });
  }
  return NextResponse.json(updated);
}
