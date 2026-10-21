import { NextResponse } from "next/server";

const base = process.env.ORCHESTRATOR_URL ?? "http://orchestrator:8090";

export async function GET() {
  const res = await fetch(`${base}/backfill`, { cache: "no-store" });
  const body = await res.text();
  return new NextResponse(body, {
    status: res.status,
    headers: { "Content-Type": "application/json" },
  });
}

export async function POST(request: Request) {
  const res = await fetch(`${base}/backfill`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: await request.text(),
  });
  const body = await res.text();
  return new NextResponse(body, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") ?? "text/plain" },
  });
}
