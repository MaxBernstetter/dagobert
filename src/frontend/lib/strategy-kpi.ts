import "server-only";

import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";

import { computeRunStatistics } from "@/lib/backtest-period-stats";
import { strategyKpiDefsForCompute } from "@/lib/strategy-kpi-catalog";
import type { StrategyKpiResult, StrategyKpiSet } from "@/lib/strategy-kpi-shared";
import { resolveDataDir } from "@/lib/data-dir";

/** Default aggregation when KPI JSON has no bar duration (legacy 3h stack). */
const DEFAULT_BAR_MS = 3 * 60 * 60 * 1000;
const DEFAULT_START_CASH = 100_000;

function barDurationMsFromBody(body: string, barOpen: number): number {
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
    /* fall through */
  }
  return DEFAULT_BAR_MS;
}

function periodsPerYear(barMs: number): number {
  if (barMs <= 0) return (365.25 * 24) / 3;
  return (365.25 * 24 * 60 * 60 * 1000) / barMs;
}

type Fill = {
  candleCloseMs: number;
  side: string;
  baseQty: number;
  quoteQty: number;
};

type Bar = { closeMs: number; close: number };
type EquityPoint = { closeMs: number; value: number };

function candleCloseFromOrderLink(link: string, playbook: string): number | null {
  const prefix = `${playbook}:`;
  const last = link.lastIndexOf(":");
  if (last <= 0 || !link.startsWith(prefix)) {
    return null;
  }
  const ms = Number.parseInt(link.slice(last + 1), 10);
  return ms > 0 ? ms : null;
}

export type StrategyKpiOptions = {
  workspaceDir?: string;
  backtestId?: string;
  /** Quote cash at t0; defaults to 100k when unset. */
  startCash?: number;
};

function loadFills(playbook: string, workspaceDir?: string): Fill[] {
  const filePath = path.join(workspaceDir ?? resolveDataDir(), "execution.sqlite");
  if (!fs.existsSync(filePath)) {
    return [];
  }
  const db = new Database(filePath, { readonly: true, fileMustExist: true });
  db.pragma("busy_timeout = 5000");
  try {
    const table = db
      .prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'fill_journal'")
      .get() as { name?: string } | undefined;
    if (!table?.name) {
      return [];
    }
    const rows = db
      .prepare(
        `SELECT order_link_id AS orderLinkId, side, base_qty AS baseQty, quote_qty AS quoteQty
         FROM fill_journal WHERE order_link_id LIKE ?`,
      )
      .all(`${playbook}:%`) as {
      orderLinkId: string;
      side: string;
      baseQty: string;
      quoteQty: string;
    }[];
    const out: Fill[] = [];
    for (const row of rows) {
      const closeMs = candleCloseFromOrderLink(row.orderLinkId, playbook);
      if (closeMs == null) continue;
      out.push({
        candleCloseMs: closeMs,
        side: row.side.toLowerCase(),
        baseQty: Number.parseFloat(row.baseQty) || 0,
        quoteQty: Number.parseFloat(row.quoteQty) || 0,
      });
    }
    out.sort((a, b) => a.candleCloseMs - b.candleCloseMs);
    return out;
  } finally {
    db.close();
  }
}

function loadBars(
  symbol: string,
  fromCloseMs: number,
  toCloseMs: number,
  opts?: StrategyKpiOptions,
): Bar[] {
  const openMin = fromCloseMs - 48 * 60 * 60 * 1000;
  const openMax = toCloseMs;
  const dbPath =
    opts?.backtestId != null
      ? path.join(resolveDataDir(), "backtests.sqlite")
      : path.join(resolveDataDir(), "technical_indicator.sqlite");
  if (!fs.existsSync(dbPath)) {
    return [];
  }
  const db = new Database(dbPath, { readonly: true, fileMustExist: true });
  db.pragma("busy_timeout = 5000");
  try {
    const rows =
      opts?.backtestId != null
        ? (db
            .prepare(
              `SELECT bar_open AS barOpen, body FROM backtest_kpi
               WHERE backtest_id = ? AND symbol = ? AND algo = 'v1' AND bar_open >= ? AND bar_open <= ?
               ORDER BY bar_open`,
            )
            .all(opts.backtestId, symbol, openMin, openMax) as { barOpen: number; body: string }[])
        : (db
            .prepare(
              `SELECT bar_open AS barOpen, body FROM kpi_revision
               WHERE symbol = ? AND algo = 'v1' AND bar_open >= ? AND bar_open <= ?
               ORDER BY bar_open`,
            )
            .all(symbol, openMin, openMax) as { barOpen: number; body: string }[]);
    const out: Bar[] = [];
    for (const row of rows) {
      const durationMs = barDurationMsFromBody(row.body, row.barOpen);
      const closeMs = row.barOpen + durationMs;
      if (closeMs < fromCloseMs || closeMs > toCloseMs) continue;
      try {
        const parsed = JSON.parse(row.body) as { close?: number };
        const close = parsed.close;
        if (close == null || close <= 0 || !Number.isFinite(close)) continue;
        out.push({ closeMs, close });
      } catch {
        continue;
      }
    }
    return out;
  } finally {
    db.close();
  }
}

function replayEquityCurves(fills: Fill[], bars: Bar[], startCash: number): [EquityPoint[], EquityPoint[]] {
  const fillAt = new Map<number, Fill[]>();
  for (const f of fills) {
    const list = fillAt.get(f.candleCloseMs) ?? [];
    list.push(f);
    fillAt.set(f.candleCloseMs, list);
  }

  let cash = startCash;
  let base = 0;
  const portfolio: EquityPoint[] = [];

  let benchCash = startCash;
  let benchBase = 0;
  const bench: EquityPoint[] = [];
  let benchInitialized = false;

  for (const bar of bars) {
    for (const f of fillAt.get(bar.closeMs) ?? []) {
      if (f.side === "buy") {
        cash -= f.quoteQty;
        base += f.baseQty;
      } else if (f.side === "sell") {
        cash += f.quoteQty;
        base -= f.baseQty;
      }
    }
    portfolio.push({ closeMs: bar.closeMs, value: cash + base * bar.close });

    if (!benchInitialized) {
      benchBase = benchCash / bar.close;
      benchCash = 0;
      benchInitialized = true;
    }
    bench.push({ closeMs: bar.closeMs, value: benchCash + benchBase * bar.close });
  }
  return [portfolio, bench];
}

function periodicReturns(curve: EquityPoint[]): number[] {
  const out: number[] = [];
  for (let i = 1; i < curve.length; i++) {
    const prev = curve[i - 1].value;
    if (prev <= 0) continue;
    out.push(curve[i].value / prev - 1);
  }
  return out;
}

function meanStd(v: number[]): [number, number] {
  if (v.length === 0) return [0, 0];
  let mean = 0;
  for (const x of v) mean += x;
  mean /= v.length;
  let sum = 0;
  for (const x of v) {
    const d = x - mean;
    sum += d * d;
  }
  return [mean, Math.sqrt(sum / v.length)];
}

function computeCAGR(start: number, end: number, years: number): number | null {
  if (start <= 0 || end <= 0 || years <= 0) return null;
  return Math.pow(end / start, 1 / years) - 1;
}

function computeMaxDrawdown(curve: EquityPoint[]): { value: number; extra?: Record<string, number> } {
  let peak = curve[0].value;
  let maxDD = 0;
  let ddPeakValue = peak;
  let troughMs = curve[0].closeMs;

  for (const p of curve) {
    if (p.value > peak) peak = p.value;
    if (peak > 0) {
      const dd = (peak - p.value) / peak;
      if (dd > maxDD) {
        maxDD = dd;
        ddPeakValue = peak;
        troughMs = p.closeMs;
      }
    }
  }

  const extra: Record<string, number> = {};
  for (const p of curve) {
    if (p.closeMs <= troughMs) continue;
    if (p.value >= ddPeakValue) {
      extra.recovery_days = (p.closeMs - troughMs) / (24 * 60 * 60 * 1000);
      break;
    }
  }
  return { value: maxDD, extra: Object.keys(extra).length ? extra : undefined };
}

function computeSharpe(curve: EquityPoint[], barMs: number): number | null {
  const rets = periodicReturns(curve);
  if (rets.length < 2) return null;
  const [mean, std] = meanStd(rets);
  if (std === 0) return null;
  return (mean / std) * Math.sqrt(periodsPerYear(barMs));
}

function computeSortino(curve: EquityPoint[], barMs: number): number | null {
  const rets = periodicReturns(curve);
  if (rets.length < 2) return null;
  let mean = 0;
  for (const r of rets) mean += r;
  mean /= rets.length;

  let downSum = 0;
  let downN = 0;
  for (const r of rets) {
    if (r < 0) {
      downSum += r * r;
      downN++;
    }
  }
  if (downN === 0) return null;
  const downDev = Math.sqrt(downSum / downN);
  if (downDev === 0) return null;
  return (mean / downDev) * Math.sqrt(periodsPerYear(barMs));
}

function inferBarMs(curve: EquityPoint[]): number {
  if (curve.length < 2) return DEFAULT_BAR_MS;
  const ms = curve[1].closeMs - curve[0].closeMs;
  return ms > 0 ? ms : DEFAULT_BAR_MS;
}

function computeValues(
  portfolio: EquityPoint[],
  benchmark: EquityPoint[],
  fromMs: number,
  toMs: number,
): StrategyKpiResult[] {
  if (portfolio.length < 2) return [];
  const startV = portfolio[0].value;
  const endV = portfolio[portfolio.length - 1].value;
  if (startV <= 0) return [];

  let years = (toMs - fromMs) / (365.25 * 24 * 60 * 60 * 1000);
  if (years <= 0) {
    years =
      (portfolio[portfolio.length - 1].closeMs - portfolio[0].closeMs) /
      (365.25 * 24 * 60 * 60 * 1000);
  }

  const barMs = inferBarMs(portfolio);
  const defs = strategyKpiDefsForCompute();
  const out: StrategyKpiResult[] = [];

  for (const def of defs) {
    switch (def.slug) {
      case "cagr": {
        const v = computeCAGR(startV, endV, years);
        if (v != null) out.push({ def, value: v });
        break;
      }
      case "max-drawdown": {
        const { value, extra } = computeMaxDrawdown(portfolio);
        out.push({ def, value, extra });
        break;
      }
      case "sharpe": {
        const v = computeSharpe(portfolio, barMs);
        if (v != null) out.push({ def, value: v });
        break;
      }
      case "active-return": {
        if (benchmark.length >= 2 && benchmark[0].value > 0) {
          const stratRet = endV / startV - 1;
          const benchRet = benchmark[benchmark.length - 1].value / benchmark[0].value - 1;
          out.push({ def, value: stratRet - benchRet });
        }
        break;
      }
      case "sortino": {
        const v = computeSortino(portfolio, barMs);
        if (v != null) out.push({ def, value: v });
        break;
      }
    }
  }
  return out;
}

/** Compute playbook KPIs for [fromMs, toMs]; 0 endpoints mean full fill span. */
export function computeStrategyKPIs(
  playbook: string,
  symbol: string,
  fromMs = 0,
  toMs = 0,
  opts?: StrategyKpiOptions,
): StrategyKpiSet {
  const fills = loadFills(playbook, opts?.workspaceDir);
  if (fills.length === 0) {
    return { playbook, symbol, fromMs: 0, toMs: 0, results: [] };
  }

  let from = fromMs;
  let to = toMs;
  if (from <= 0) from = fills[0].candleCloseMs;
  if (to <= 0) to = fills[fills.length - 1].candleCloseMs;
  if (from > to) {
    throw new Error("invalid kpi range");
  }

  const bars = loadBars(symbol, from, to, opts);
  if (bars.length < 2) {
    return { playbook, symbol, fromMs: from, toMs: to, results: [] };
  }

  const startCash =
    opts?.startCash != null && opts.startCash > 0 ? opts.startCash : DEFAULT_START_CASH;
  const [portfolio, bench] = replayEquityCurves(fills, bars, startCash);
  if (portfolio.length < 2) {
    return { playbook, symbol, fromMs: from, toMs: to, results: [] };
  }

  const startV = portfolio[0].value;
  const endV = portfolio[portfolio.length - 1].value;
  const benchStart = bench[0]?.value ?? 0;
  const benchEnd = bench[bench.length - 1]?.value ?? 0;
  const comparison =
    startV > 0 && benchStart > 0
      ? {
          startCash: startV,
          strategyReturn: endV / startV - 1,
          buyHoldReturn: benchEnd / benchStart - 1,
        }
      : undefined;

  const statistics = computeRunStatistics(portfolio, fills, from, to);

  return {
    playbook,
    symbol,
    fromMs: from,
    toMs: to,
    results: computeValues(portfolio, bench, from, to),
    comparison,
    statistics,
  };
}
