import type { ActivityRow, JournalRow } from "@/types/read";
import { playbookFromOrderLinkId } from "@/lib/strategy-groups";

export type PortfolioPoint = { time: number; value: number };
export type TradeMarker = { time: number; side: "buy" | "sell" };

/** Fills from activity rows (same candle_close as the decisions table). */
export function tradeMarkersFromActivity(rows: ActivityRow[]): TradeMarker[] {
  const out: TradeMarker[] = [];
  for (const row of rows) {
    if (row.kind !== "fill") continue;
    const raw = (row.decision ?? row.side ?? "").toLowerCase();
    if (raw !== "buy" && raw !== "sell") continue;
    if (row.candleClose <= 0) continue;
    out.push({
      time: Math.floor(row.candleClose / 1000),
      side: raw as "buy" | "sell",
    });
  }
  return out.sort((a, b) => a.time - b.time);
}

function candleCloseMsFromLink(orderLinkId: string): number | null {
  const last = orderLinkId.lastIndexOf(":");
  if (last <= 0) return null;
  const ms = Number(orderLinkId.slice(last + 1));
  return Number.isFinite(ms) ? ms : null;
}

/** Replay fill_journal rows into a mark-to-market equity curve (quote + base × price). */
export function portfolioSeriesForPlaybook(
  journals: JournalRow[],
  playbook: string,
  startingQuote = 50_000
): { points: PortfolioPoint[]; markers: TradeMarker[] } {
  const fills = journals
    .filter((j) => playbookFromOrderLinkId(j.orderLinkId) === playbook)
    .map((j) => {
      const closeMs = candleCloseMsFromLink(j.orderLinkId);
      return closeMs != null ? { journal: j, closeMs } : null;
    })
    .filter((x): x is { journal: JournalRow; closeMs: number } => x != null)
    .sort((a, b) => a.closeMs - b.closeMs);

  if (fills.length === 0) {
    return { points: [], markers: [] };
  }

  let cash = startingQuote;
  let base = 0;
  const points: PortfolioPoint[] = [];
  const markers: TradeMarker[] = [];

  const firstSec = Math.floor(fills[0].closeMs / 1000);
  points.push({ time: firstSec - 1, value: startingQuote });

  for (const { journal: j, closeMs } of fills) {
    const baseQty = Number(j.baseQty);
    const quoteQty = Number(j.quoteQty);
    const price = Number(j.price);
    if (!Number.isFinite(baseQty) || !Number.isFinite(quoteQty) || !Number.isFinite(price)) {
      continue;
    }
    const time = Math.floor(closeMs / 1000);
    const side = j.side.toLowerCase();
    if (side === "buy") {
      cash -= quoteQty;
      base += baseQty;
      markers.push({ time, side: "buy" });
    } else if (side === "sell") {
      cash += quoteQty;
      base -= baseQty;
      markers.push({ time, side: "sell" });
    }
    points.push({ time, value: cash + base * price });
  }

  return { points, markers };
}
