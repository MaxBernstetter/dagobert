import { NextResponse } from "next/server";
import { readPosition } from "@/lib/read";

const LABELS: Record<string, string> = {
  BTCUSDT: "BTC",
  ETHUSDT: "ETH",
};

export async function GET(request: Request) {
  const url = new URL(request.url);
  const symbol = url.searchParams.get("symbol") ?? "";
  const label = LABELS[symbol];
  if (!label) {
    return NextResponse.json({ error: "unknown position" }, { status: 400 });
  }
  const algo = url.searchParams.get("algo") || "v1";
  return NextResponse.json(readPosition(symbol, label, algo));
}
