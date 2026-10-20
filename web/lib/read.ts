import "server-only";

import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";

// DATA_DIR defaults to ./data under the web app working directory; absolute paths are used as-is.
function dataDir(): string {
  const raw = process.env.DATA_DIR ?? "./data";
  return path.isAbsolute(raw) ? raw : path.join(process.cwd(), raw);
}

const EXECUTION_SCHEMA = `
CREATE TABLE IF NOT EXISTS order_result (
  order_link_id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS balance (
  currency TEXT PRIMARY KEY,
  cash_balance TEXT NOT NULL,
  transaction_time INTEGER NOT NULL
);
`;

export type BalanceRow = {
  currency: string;
  cashBalance: string;
  transactionTime: number;
};

export type OrderKind = "fill" | "rejection" | "no-trade" | "pending";

export type ActivityRow = {
  orderLinkId: string;
  playbook: string;
  symbol: string;
  candleClose: number;
  kind: OrderKind | "not-yet-filled";
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

export type ActionBody = {
  decision?: string;
  size?: number | null;
  exit?: unknown;
  rule?: string;
  written_at?: string;
};

type OrderBody = Record<string, unknown>;

function openExecutionDb(dir: string): Database.Database {
  fs.mkdirSync(dir, { recursive: true });
  const dbPath = path.join(dir, "execution.sqlite");
  const db = new Database(dbPath, { readonly: false });
  db.exec(EXECUTION_SCHEMA);
  return db;
}

function openStrategyDbReadonly(dir: string): Database.Database | null {
  const dbPath = path.join(dir, "strategy.sqlite");
  if (!fs.existsSync(dbPath)) {
    return null;
  }
  return new Database(dbPath, { readonly: true });
}

function parseOrderLinkId(orderLinkId: string): {
  playbook: string;
  symbol: string;
  candleClose: number;
} | null {
  const last = orderLinkId.lastIndexOf(":");
  if (last <= 0) return null;
  const candleClose = Number(orderLinkId.slice(last + 1));
  if (!Number.isFinite(candleClose)) return null;
  const rest = orderLinkId.slice(0, last);
  const sep = rest.lastIndexOf(":");
  if (sep <= 0) return null;
  return {
    playbook: rest.slice(0, sep),
    symbol: rest.slice(sep + 1),
    candleClose,
  };
}

function actionKey(playbook: string, symbol: string, candleClose: number): string {
  return `${playbook}\0${symbol}\0${candleClose}`;
}

function readActions(
  db: Database.Database | null
): Map<string, ActionBody & { playbook: string; symbol: string; candleClose: number }> {
  const map = new Map<
    string,
    ActionBody & { playbook: string; symbol: string; candleClose: number }
  >();
  if (!db) return map;
  const rows = db
    .prepare(
      `SELECT playbook, symbol, candle_close, body FROM action ORDER BY candle_close DESC`
    )
    .all() as { playbook: string; symbol: string; candle_close: number; body: string }[];

  for (const row of rows) {
    let body: ActionBody = {};
    try {
      body = JSON.parse(row.body) as ActionBody;
    } catch {
      body = {};
    }
    map.set(actionKey(row.playbook, row.symbol, row.candle_close), {
      ...body,
      playbook: row.playbook,
      symbol: row.symbol,
      candleClose: row.candle_close,
    });
  }
  return map;
}

function keysFromBody(body: OrderBody, orderLinkId: string): {
  playbook: string;
  symbol: string;
  candleClose: number;
} {
  const playbook = String(body.playbook ?? "");
  const symbol = String(body.symbol ?? "");
  const candleClose = Number(body.candleClose);
  if (playbook && symbol && Number.isFinite(candleClose)) {
    return { playbook, symbol, candleClose };
  }
  const parsed = parseOrderLinkId(orderLinkId);
  if (parsed) return parsed;
  return { playbook, symbol, candleClose: Number.isFinite(candleClose) ? candleClose : 0 };
}

function mergeActivity(
  orderLinkId: string,
  kind: ActivityRow["kind"],
  body: OrderBody,
  action:
    | (ActionBody & { playbook: string; symbol: string; candleClose: number })
    | undefined
): ActivityRow {
  const keys = keysFromBody(body, orderLinkId);
  const playbook = keys.playbook || action?.playbook || "";
  const symbol = keys.symbol || action?.symbol || "";
  const candleClose = keys.candleClose || action?.candleClose || 0;

  return {
    orderLinkId,
    playbook,
    symbol,
    candleClose,
    kind,
    decision: action?.decision ?? (body.decision != null ? String(body.decision) : null),
    rule: action?.rule ?? null,
    size: action?.size ?? (body.size != null ? Number(body.size) : null),
    exit: action?.exit ?? null,
    writtenAt: action?.written_at ?? null,
    orderId: body.orderId != null ? String(body.orderId) : null,
    orderStatus: body.orderStatus != null ? String(body.orderStatus) : null,
    venueSymbol: body.venueSymbol != null ? String(body.venueSymbol) : null,
    side: body.side != null ? String(body.side) : null,
    qty: body.qty != null ? String(body.qty) : null,
    cumExecQty: body.cumExecQty != null ? String(body.cumExecQty) : null,
    cumExecValue: body.cumExecValue != null ? String(body.cumExecValue) : null,
    avgPrice: body.avgPrice != null ? String(body.avgPrice) : null,
    marketUnit: body.marketUnit != null ? String(body.marketUnit) : null,
    retCode: body.retCode != null ? Number(body.retCode) : null,
    retMsg: body.retMsg != null ? String(body.retMsg) : null,
  };
}

export function readBalances(): BalanceRow[] {
  const dir = dataDir();
  let db: Database.Database;
  try {
    db = openExecutionDb(dir);
  } catch {
    return [];
  }
  try {
    const rows = db
      .prepare(
        `SELECT currency, cash_balance, transaction_time FROM balance ORDER BY currency`
      )
      .all() as {
      currency: string;
      cash_balance: string;
      transaction_time: number;
    }[];
    return rows.map((r) => ({
      currency: r.currency,
      cashBalance: r.cash_balance,
      transactionTime: r.transaction_time,
    }));
  } finally {
    db.close();
  }
}

export function readActivity(): ActivityRow[] {
  const dir = dataDir();
  const strategyDb = openStrategyDbReadonly(dir);
  const actions = readActions(strategyDb);
  if (strategyDb) strategyDb.close();

  const byLink = new Map<string, ActivityRow>();
  const seenActions = new Set<string>();

  try {
    const execDb = openExecutionDb(dir);
    try {
      const orders = execDb
        .prepare(`SELECT order_link_id, kind, body FROM order_result`)
        .all() as { order_link_id: string; kind: string; body: string }[];

      for (const row of orders) {
        let body: OrderBody = {};
        try {
          body = JSON.parse(row.body) as OrderBody;
        } catch {
          body = {};
        }
        const kind = row.kind as OrderKind;
        const keys = keysFromBody(body, row.order_link_id);
        const ak = actionKey(keys.playbook, keys.symbol, keys.candleClose);
        seenActions.add(ak);
        byLink.set(
          row.order_link_id,
          mergeActivity(row.order_link_id, kind, body, actions.get(ak))
        );
      }
    } finally {
      execDb.close();
    }
  } catch {
    // Missing or unreadable execution store: strategy-only rows still appear below.
  }

  for (const [ak, action] of actions) {
    if (seenActions.has(ak)) continue;
    const orderLinkId = `${action.playbook}:${action.symbol}:${action.candleClose}`;
    byLink.set(
      orderLinkId,
      mergeActivity(orderLinkId, "not-yet-filled", {}, action)
    );
  }

  return [...byLink.values()].sort((a, b) => b.candleClose - a.candleClose);
}

// pipeline.json: {service, status, start, finish, retries, error, candleClose}[]
export type PipelineRow = {
  service: string;
  status: string;
  start: string | null;
  finish: string | null;
  retries: number | null;
  error: string | null;
  candleClose: number | null;
};

const PIPELINE_PLACEHOLDERS: PipelineRow[] = [
  {
    service: "ingest",
    status: "unavailable",
    start: null,
    finish: null,
    retries: null,
    error: null,
    candleClose: null,
  },
  {
    service: "process",
    status: "unavailable",
    start: null,
    finish: null,
    retries: null,
    error: null,
    candleClose: null,
  },
  {
    service: "strategy",
    status: "unavailable",
    start: null,
    finish: null,
    retries: null,
    error: null,
    candleClose: null,
  },
  {
    service: "execution",
    status: "unavailable",
    start: null,
    finish: null,
    retries: null,
    error: null,
    candleClose: null,
  },
];

export function readPipeline(): PipelineRow[] {
  const filePath = path.join(dataDir(), "pipeline.json");
  if (!fs.existsSync(filePath)) {
    return PIPELINE_PLACEHOLDERS;
  }
  try {
    const raw = fs.readFileSync(filePath, "utf8");
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return PIPELINE_PLACEHOLDERS;
    }
    return parsed.map((item) => {
      const row = item as Record<string, unknown>;
      return {
        service: String(row.service ?? ""),
        status: String(row.status ?? ""),
        start: row.start != null ? String(row.start) : null,
        finish: row.finish != null ? String(row.finish) : null,
        retries: row.retries != null ? Number(row.retries) : null,
        error: row.error != null ? String(row.error) : null,
        candleClose:
          row.candleClose != null ? Number(row.candleClose) : null,
      };
    });
  } catch {
    return PIPELINE_PLACEHOLDERS;
  }
}
