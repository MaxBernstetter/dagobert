// Row shapes of the pipeline and execution records shown in the SPA.
// Copied from the former server reader (SQLite); the SPA gets these rows from the Experiment API.

export type JournalRow = {
  orderLinkId: string;
  venue: string;
  venueSymbol: string;
  side: string;
  baseQty: string;
  quoteQty: string;
  price: string;
  fee: string;
  feeCurrency: string;
  priceSource: string;
};

export type OrderKind = "fill" | "rejection" | "no-trade" | "pending";

export type ActivityRow = {
  orderLinkId: string;
  playbook: string;
  symbol: string;
  candleClose: number;
  kind: OrderKind | "not-yet-filled";
  /** Layer-1 signal (calculation-signal). */
  signalDecision: string | null;
  signalRule: string | null;
  /** Layer-2 sized market action. */
  decision: string | null;
  rule: string | null;
  size: number | null;
  exit: unknown;
  writtenAt: string | null;
  orderId: string | null;
  orderStatus: string | null;
  venueSymbol: string | null;
  side: string | null;
  qty: string | null;
  cumExecQty: string | null;
  cumExecValue: string | null;
  avgPrice: string | null;
  marketUnit: string | null;
  retCode: number | null;
  retMsg: string | null;
};

export type LiveState = {
  stage: string;
  symbol: string;
  candleClose: number;
  since: string | null;
  lastStage: string;
  lastExit: number | null;
  lastFinished: string | null;
};
