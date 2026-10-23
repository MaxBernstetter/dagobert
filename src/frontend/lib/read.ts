import "server-only";

import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";

import {
  type PlaybookChartIndicator,
  indicatorValuesFromKpiBody,
} from "@/lib/playbook-indicators";

import { resolveDataDir } from "@/lib/data-dir";

function dataDir(): string {
  return resolveDataDir();
}

function openSqliteReadonly(dbPath: string): Database.Database {
  const db = new Database(dbPath, { readonly: true, fileMustExist: true });
  db.pragma("busy_timeout = 5000");
  return db;
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

export function readJournals(workspaceDir?: string): JournalRow[] {
  const filePath = path.join(workspaceDir ?? dataDir(), "execution.sqlite");
  if (!fs.existsSync(filePath)) return [];
  const db = new Database(filePath, { readonly: true, fileMustExist: true });
  try {
    const table = db.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'fill_journal'").get() as { name?: string } | undefined;
    if (table?.name) {
      const rows = db.prepare(
        "SELECT order_link_id AS orderLinkId, venue, venue_symbol AS venueSymbol, side, base_qty AS baseQty, quote_qty AS quoteQty, price, fee, fee_currency AS feeCurrency, price_source AS priceSource FROM fill_journal"
      ).all() as JournalRow[];
      if (rows.length > 0) return rows;
    }
    const fills = db.prepare("SELECT body FROM order_result WHERE kind = 'fill'").all() as { body: string }[];
    return fills.map((row) => {
      const body = JSON.parse(row.body) as Record<string, string>;
      return {
        orderLinkId: body.orderLinkId ?? "",
        venue: "bybit",
        venueSymbol: body.venueSymbol ?? "",
        side: body.side ?? "",
        baseQty: body.cumExecQty ?? "",
        quoteQty: body.cumExecValue ?? "",
        price: body.avgPrice ?? "",
        fee: body.cumExecFee ?? "",
        feeCurrency: body.feeCurrency ?? "USDC",
        priceSource: "binance",
      };
    });
  } catch {
    return [];
  } finally {
    db.close();
  }
}

export type PositionRow = {
  venueSymbol: string;
  base: string;
  quantity: string;
  costQuote: string;
  avgPrice: string;
};

export function readPositions(): PositionRow[] {
  const lots = new Map<string, { base: number; cost: number }>();
  for (const row of readJournals()) {
    const qty = Number(row.baseQty);
    const quote = Number(row.quoteQty);
    if (!Number.isFinite(qty) || !Number.isFinite(quote) || !row.venueSymbol) continue;
    const lot = lots.get(row.venueSymbol) ?? { base: 0, cost: 0 };
    if (row.side.toLowerCase() === "buy") {
      lot.base += qty;
      lot.cost += quote;
    } else if (lot.base > 0) {
      const avg = lot.cost / lot.base;
      const closed = Math.min(qty, lot.base);
      lot.base -= closed;
      lot.cost -= avg * closed;
      if (lot.base <= 1e-12) {
        lot.base = 0;
        lot.cost = 0;
      }
    }
    lots.set(row.venueSymbol, lot);
  }
  return [...lots.entries()]
    .filter(([, lot]) => lot.base > 0)
    .map(([venueSymbol, lot]) => ({
      venueSymbol,
      base: venueSymbol.replace(/USDC$|USDT$/, ""),
      quantity: lot.base.toPrecision(8),
      costQuote: lot.cost.toPrecision(8),
      avgPrice: (lot.cost / lot.base).toPrecision(8),
    }));
}

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

export type ActionBody = {
  decision?: string;
  size?: number | null;
  exit?: unknown;
  rule?: string;
  signal_rule?: string;
  written_at?: string;
};

export type SignalBody = {
  decision?: string;
  rule?: string;
};

type OrderBody = Record<string, unknown>;

function openExecutionDb(dir: string): Database.Database {
  fs.mkdirSync(dir, { recursive: true });
  const dbPath = path.join(dir, "execution.sqlite");
  const db = new Database(dbPath, { readonly: false });
  db.exec(EXECUTION_SCHEMA);
  return db;
}

function signalSqlitePath(dir: string): string | null {
  const current = path.join(dir, "calculation_signal.sqlite");
  if (fs.existsSync(current)) return current;
  const legacy = path.join(dir, "strategy.sqlite");
  if (fs.existsSync(legacy)) return legacy;
  return null;
}

function openStrategyDbReadonly(dir: string): Database.Database | null {
  const dbPath = signalSqlitePath(dir);
  if (!dbPath) {
    return null;
  }
  try {
    return openSqliteReadonly(dbPath);
  } catch {
    return null;
  }
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

function readSignals(
  db: Database.Database | null
): Map<string, SignalBody & { playbook: string; symbol: string; candleClose: number }> {
  const map = new Map<
    string,
    SignalBody & { playbook: string; symbol: string; candleClose: number }
  >();
  if (!db) return map;
  let rows: { playbook: string; symbol: string; candle_close: number; body: string }[] = [];
  try {
    rows = db
      .prepare(
        `SELECT playbook, symbol, candle_close, body FROM signal ORDER BY candle_close DESC`
      )
      .all() as { playbook: string; symbol: string; candle_close: number; body: string }[];
  } catch {
    return map;
  }
  for (const row of rows) {
    let body: SignalBody = {};
    try {
      body = JSON.parse(row.body) as SignalBody;
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

function readActions(
  db: Database.Database | null
): Map<string, ActionBody & { playbook: string; symbol: string; candleClose: number }> {
  const map = new Map<
    string,
    ActionBody & { playbook: string; symbol: string; candleClose: number }
  >();
  if (!db) return map;
  let rows: { playbook: string; symbol: string; candle_close: number; body: string }[] = [];
  try {
    rows = db
      .prepare(
        `SELECT playbook, symbol, candle_close, body FROM action ORDER BY candle_close DESC`
      )
      .all() as { playbook: string; symbol: string; candle_close: number; body: string }[];
  } catch {
    return map;
  }

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
    | undefined,
  signal:
    | (SignalBody & { playbook: string; symbol: string; candleClose: number })
    | undefined
): ActivityRow {
  const keys = keysFromBody(body, orderLinkId);
  const playbook = keys.playbook || action?.playbook || signal?.playbook || "";
  const symbol = keys.symbol || action?.symbol || signal?.symbol || "";
  const candleClose = keys.candleClose || action?.candleClose || signal?.candleClose || 0;

  return {
    orderLinkId,
    playbook,
    symbol,
    candleClose,
    kind,
    signalDecision: signal?.decision ?? null,
    signalRule: signal?.rule ?? null,
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

export function readActivity(workspaceDir?: string): ActivityRow[] {
  const dir = workspaceDir ?? dataDir();
  const strategyDb = openStrategyDbReadonly(dir);
  const signals = readSignals(strategyDb);
  const actions = readActions(strategyDb);
  if (strategyDb) strategyDb.close();

  const byLink = new Map<string, ActivityRow>();
  const seenKeys = new Set<string>();

  const execPath = path.join(dir, "execution.sqlite");
  try {
    if (!fs.existsSync(execPath)) {
      throw new Error("no execution store");
    }
    const execDb = openSqliteReadonly(execPath);
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
        seenKeys.add(ak);
        byLink.set(
          row.order_link_id,
          mergeActivity(row.order_link_id, kind, body, actions.get(ak), signals.get(ak))
        );
      }
    } finally {
      execDb.close();
    }
  } catch {
    // Missing or unreadable execution store: strategy-only rows still appear below.
  }

  for (const [ak, action] of actions) {
    if (seenKeys.has(ak)) continue;
    seenKeys.add(ak);
    const orderLinkId = `${action.playbook}:${action.symbol}:${action.candleClose}`;
    byLink.set(
      orderLinkId,
      mergeActivity(orderLinkId, "not-yet-filled", {}, action, signals.get(ak))
    );
  }

  for (const [ak, signal] of signals) {
    if (seenKeys.has(ak)) continue;
    seenKeys.add(ak);
    const orderLinkId = `${signal.playbook}:${signal.symbol}:${signal.candleClose}`;
    byLink.set(
      orderLinkId,
      mergeActivity(orderLinkId, "not-yet-filled", {}, undefined, signal)
    );
  }

  return [...byLink.values()].sort((a, b) => b.candleClose - a.candleClose);
}

export type LiveState = {
  stage: string;
  symbol: string;
  candleClose: number;
  since: string | null;
  lastStage: string;
  lastExit: number | null;
  lastFinished: string | null;
};

export type DataLakeStats = {
  hours: number;
  newestHour: number | null;
  symbols: number;
  kpiRows: number;
  newestBar: number | null;
  parquetBytes: number;
  parquetUpdated: string | null;
};

export type StrategyLakeStats = {
  signals: number;
  actions: number;
  fills: number;
  playbooks: number;
};

export type LakeStats = DataLakeStats & StrategyLakeStats;

export function readLive(): LiveState {
  const empty: LiveState = {
    stage: "",
    symbol: "",
    candleClose: 0,
    since: null,
    lastStage: "",
    lastExit: null,
    lastFinished: null,
  };
  const filePath = path.join(dataDir(), "live.json");
  if (!fs.existsSync(filePath)) return empty;
  try {
    const row = JSON.parse(fs.readFileSync(filePath, "utf8")) as Record<string, unknown>;
    return {
      stage: String(row.stage ?? ""),
      symbol: String(row.symbol ?? ""),
      candleClose: Number(row.candleClose ?? 0),
      since: row.since != null ? String(row.since) : null,
      lastStage: String(row.lastStage ?? ""),
      lastExit: row.lastExit != null ? Number(row.lastExit) : null,
      lastFinished: row.lastFinished != null ? String(row.lastFinished) : null,
    };
  } catch {
    return empty;
  }
}

function countQuery(file: string, sql: string): { n: number; newest: number | null; symbols: number } {
  const filePath = path.join(dataDir(), file);
  if (!fs.existsSync(filePath)) return { n: 0, newest: null, symbols: 0 };
  const db = new Database(filePath, { readonly: true, fileMustExist: true });
  try {
    const row = db.prepare(sql).get() as { n: number; newest: number | null; symbols?: number };
    return { n: row.n ?? 0, newest: row.newest ?? null, symbols: row.symbols ?? 0 };
  } catch {
    return { n: 0, newest: null, symbols: 0 };
  } finally {
    db.close();
  }
}

export function readLake(): LakeStats {
  const hours = countQuery(
    "ingest.sqlite",
    "SELECT COUNT(*) AS n, MAX(open_time) AS newest, COUNT(DISTINCT symbol) AS symbols FROM stored"
  );
  const kpi = countQuery(
    "technical_indicator.sqlite",
    "SELECT COUNT(*) AS n, MAX(bar_open) AS newest FROM kpi"
  );
  const signalFile = signalSqlitePath(dataDir());
  const signals = signalFile
    ? countQuery(path.basename(signalFile), "SELECT COUNT(*) AS n, MAX(candle_close) AS newest FROM signal")
    : { n: 0, newest: null, symbols: 0 };
  const actions = signalFile
    ? countQuery(path.basename(signalFile), "SELECT COUNT(*) AS n, MAX(candle_close) AS newest FROM action")
    : { n: 0, newest: null, symbols: 0 };
  const playbooks = signalFile ? countDistinctPlaybooks(path.basename(signalFile)) : 0;
  const fills = countQuery(
    "execution.sqlite",
    "SELECT COUNT(*) AS n, NULL AS newest FROM order_result WHERE kind = 'fill'"
  );
  let parquetBytes = 0;
  let parquetUpdated: string | null = null;
  const parquetDir = path.join(dataDir(), "parquet");
  if (fs.existsSync(parquetDir)) {
    for (const name of fs.readdirSync(parquetDir)) {
      if (!name.endsWith(".parquet")) continue;
      const st = fs.statSync(path.join(parquetDir, name));
      parquetBytes += st.size;
      const iso = st.mtime.toISOString();
      if (!parquetUpdated || iso > parquetUpdated) parquetUpdated = iso;
    }
  }
  return {
    hours: hours.n,
    newestHour: hours.newest,
    symbols: hours.symbols,
    kpiRows: kpi.n,
    newestBar: kpi.newest,
    signals: signals.n,
    actions: actions.n,
    fills: fills.n,
    playbooks,
    parquetBytes,
    parquetUpdated,
  };
}

export function readDataLake(): DataLakeStats {
  const lake = readLake();
  return {
    hours: lake.hours,
    newestHour: lake.newestHour,
    symbols: lake.symbols,
    kpiRows: lake.kpiRows,
    newestBar: lake.newestBar,
    parquetBytes: lake.parquetBytes,
    parquetUpdated: lake.parquetUpdated,
  };
}

function countDistinctPlaybooks(sqliteFile: string): number {
  const filePath = path.join(dataDir(), sqliteFile);
  if (!fs.existsSync(filePath)) return 0;
  const db = new Database(filePath, { readonly: true, fileMustExist: true });
  try {
    const row = db.prepare("SELECT COUNT(DISTINCT playbook) AS n FROM action").get() as { n: number };
    return row?.n ?? 0;
  } catch {
    return 0;
  } finally {
    db.close();
  }
}

export function readStrategyLake(): StrategyLakeStats {
  const lake = readLake();
  return {
    signals: lake.signals,
    actions: lake.actions,
    fills: lake.fills,
    playbooks: lake.playbooks,
  };
}

const BAR_MS = 10_800_000;
const MAX_BAR_MS = 48 * 60 * 60 * 1000;

function barDurationMsFromKpiBody(body: string): number {
  try {
    const parsed = JSON.parse(body) as { open_time?: number; close_time?: number };
    if (
      parsed.open_time != null &&
      parsed.close_time != null &&
      parsed.close_time >= parsed.open_time
    ) {
      return parsed.close_time - parsed.open_time + 1;
    }
  } catch {
    /* legacy bodies */
  }
  return BAR_MS;
}

/** Unix seconds at bar close — prefers KPI `close_time` (works for 3h, 6h, etc.). */
function barCloseSecFromKpi(barOpenMs: number, body: string): number {
  try {
    const parsed = JSON.parse(body) as { close_time?: number };
    if (parsed.close_time != null && parsed.close_time > 0) {
      return Math.floor(parsed.close_time / 1000);
    }
  } catch {
    /* fall through */
  }
  const dur = barDurationMsFromKpiBody(body);
  return Math.floor((barOpenMs + dur) / 1000);
}

export type PlaybookChartBar = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  indicators: Record<string, number | null>;
};

/** KPI bars for strategy charts (price + playbook technical indicators) in a unix-second range. */
export function readPlaybookChartBars(
  symbol: string,
  algo: string,
  fromSec: number,
  toSec: number,
  indicators: PlaybookChartIndicator[],
  backtestId?: string,
): PlaybookChartBar[] {
  const fromMs = fromSec * 1000;
  const toMs = toSec * 1000;
  const kpiPath = backtestId
    ? path.join(dataDir(), "backtests.sqlite")
    : path.join(dataDir(), "technical_indicator.sqlite");
  if (!fs.existsSync(kpiPath) || fromSec > toSec) return [];

  const db = new Database(kpiPath, { readonly: true, fileMustExist: true });
  const out: PlaybookChartBar[] = [];
  try {
    const openMin = fromMs - MAX_BAR_MS;
    const openMax = toMs;
    let rows: { bar_open: number; body: string }[] = [];
    if (backtestId) {
      rows = db
        .prepare(
          `SELECT bar_open, body FROM backtest_kpi
           WHERE backtest_id = ? AND symbol = ? AND algo = ? AND bar_open >= ? AND bar_open <= ?
           ORDER BY bar_open ASC`,
        )
        .all(backtestId, symbol, algo, openMin, openMax) as { bar_open: number; body: string }[];
    } else {
      try {
        rows = db
          .prepare(
            `SELECT bar_open, body FROM kpi_revision
             WHERE symbol = ? AND algo = ? AND bar_open >= ? AND bar_open <= ?
             ORDER BY bar_open ASC`,
          )
          .all(symbol, algo, openMin, openMax) as { bar_open: number; body: string }[];
      } catch {
        rows = db
          .prepare(
            `SELECT bar_open, body FROM kpi
             WHERE symbol = ? AND bar_open >= ? AND bar_open <= ?
             ORDER BY bar_open ASC`,
          )
          .all(symbol, openMin, openMax) as { bar_open: number; body: string }[];
      }
    }
    for (const row of rows) {
      const body = JSON.parse(row.body) as {
        open: number;
        high: number;
        low: number;
        close: number;
      };
      const time = barCloseSecFromKpi(row.bar_open, row.body);
      if (time < fromSec || time > toSec) continue;
      out.push({
        time,
        open: body.open,
        high: body.high,
        low: body.low,
        close: body.close,
        indicators: indicatorValuesFromKpiBody(row.body, indicators),
      });
    }
  } catch {
    return [];
  } finally {
    db.close();
  }
  return out;
}

export type PositionView = {
  symbol: string;
  label: string;
  provider: string;
  algo: string;
  start: number;
  cells: string;
  bars: {
    time: number;
    open: number;
    high: number;
    low: number;
    close: number;
    sma3: number | null;
    sma6: number | null;
    rsi: number | null;
  }[];
};

export function readAlgos(): string[] {
  const filePath = path.join(dataDir(), "technical_indicator.sqlite");
  if (!fs.existsSync(filePath)) return ["v1"];
  const db = new Database(filePath, { readonly: true, fileMustExist: true });
  try {
    const rows = db.prepare("SELECT DISTINCT algo FROM kpi_revision ORDER BY algo").all() as { algo: string }[];
    const algos = rows.map((row) => row.algo).filter(Boolean);
    return algos.length > 0 ? algos : ["v1"];
  } catch {
    return ["v1"];
  } finally {
    db.close();
  }
}

export function readPosition(symbol: string, label: string, algo = "v1"): PositionView {
  const empty: PositionView = { symbol, label, provider: "binance", algo, start: 0, cells: "", bars: [] };
  const kpiPath = path.join(dataDir(), "technical_indicator.sqlite");
  const ingestPath = path.join(dataDir(), "ingest.sqlite");
  if (!fs.existsSync(kpiPath) && !fs.existsSync(ingestPath)) return empty;

  let minHour: number | null = null;
  let maxHour: number | null = null;
  if (fs.existsSync(ingestPath)) {
    const db = new Database(ingestPath, { readonly: true, fileMustExist: true });
    try {
      const row = db.prepare("SELECT MIN(open_time) AS min, MAX(open_time) AS max FROM stored WHERE symbol = ?").get(symbol) as {
        min: number | null;
        max: number | null;
      };
      minHour = row?.min ?? null;
      maxHour = row?.max ?? null;
    } catch {
      minHour = null;
    } finally {
      db.close();
    }
  }

  const have = new Map<number, string>();
  const bars: PositionView["bars"] = [];
  if (fs.existsSync(kpiPath)) {
    const db = new Database(kpiPath, { readonly: true, fileMustExist: true });
    try {
      const rows = db
        .prepare(
          "SELECT bar_open, body, origin AS source FROM kpi_revision WHERE symbol = ? AND algo = ? ORDER BY bar_open"
        )
        .all(symbol, algo) as {
        bar_open: number;
        body: string;
        source: string | null;
      }[];
      for (const row of rows) {
        have.set(row.bar_open, row.source === "live" ? "g" : "y");
        bars.push(barPoint(row.bar_open, row.body));
      }
    } catch {
      const rows = db.prepare("SELECT bar_open, body FROM kpi WHERE symbol = ? ORDER BY bar_open").all(symbol) as {
        bar_open: number;
        body: string;
      }[];
      for (const row of rows) {
        have.set(row.bar_open, "y");
        bars.push(barPoint(row.bar_open, row.body));
      }
    } finally {
      db.close();
    }
  }

  const opens = [...have.keys()];
  const first = minHour ?? (opens.length ? opens[0] : null);
  const last = maxHour ?? (opens.length ? opens[opens.length - 1] : null);
  if (first == null || last == null) return empty;
  const start = first - (first % BAR_MS);
  const end = last - (last % BAR_MS);
  let cells = "";
  for (let t = start; t <= end; t += BAR_MS) cells += have.get(t) ?? "r";
  return { symbol, label, provider: "binance", algo, start, cells, bars };
}

function barPoint(open: number, body: string): PositionView["bars"][number] {
  const row = JSON.parse(body) as {
    open: number;
    high: number;
    low: number;
    close: number;
    results?: { def: { type?: string; method?: string; time_frame_hr?: number } | string; value: number }[];
  };
  let sma3: number | null = null;
  let sma6: number | null = null;
  let rsi: number | null = null;
  for (const result of row.results ?? []) {
    const def = typeof result.def === "string" ? JSON.parse(result.def) : result.def;
    if (def.type === "MovingAverage" && def.method === "sma" && def.time_frame_hr === 3) sma3 = result.value;
    if (def.type === "MovingAverage" && def.method === "sma" && def.time_frame_hr === 6) sma6 = result.value;
    if (def.type === "Rsi") rsi = result.value;
  }
  return {
    time: barCloseSecFromKpi(open, body),
    open: row.open,
    high: row.high,
    low: row.low,
    close: row.close,
    sma3,
    sma6,
    rsi,
  };
}
