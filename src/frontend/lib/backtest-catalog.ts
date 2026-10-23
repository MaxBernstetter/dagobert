import "server-only";

import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";

import { resolveDataDir } from "@/lib/data-dir";
import {
  alignMsToHour,
  defaultBacktestSettings,
  playbookIdForKpi,
  utcMidnightFromMs,
  type BacktestRecord,
  type BacktestSettings,
} from "@/lib/backtest-shared";

export type { BacktestRecord, BacktestSettings };
export { playbookIdForKpi };

const BAR_MS = 3 * 60 * 60 * 1000;

/** Align draft range to KPI rows in the lake when available (server-only). */
function lakeRangeForSymbol(symbol: string): { fromMs: number; toMs: number } | null {
  const tiPath = path.join(resolveDataDir(), "technical_indicator.sqlite");
  if (!fs.existsSync(tiPath)) {
    return null;
  }
  const db = new Database(tiPath, { readonly: true, fileMustExist: true });
  db.pragma("busy_timeout = 5000");
  try {
    const row = db
      .prepare(
        `SELECT MIN(bar_open) AS minOpen, MAX(bar_open) AS maxOpen
         FROM kpi_revision WHERE symbol = ? AND algo = 'v1'`,
      )
      .get(symbol) as { minOpen: number | null; maxOpen: number | null } | undefined;
    if (row?.minOpen == null || row?.maxOpen == null) {
      return null;
    }
    return {
      fromMs: utcMidnightFromMs(row.minOpen),
      toMs: alignMsToHour(row.maxOpen + BAR_MS),
    };
  } finally {
    db.close();
  }
}

const SCHEMA = `
CREATE TABLE IF NOT EXISTS backtest (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  workspace_rel_path TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  error TEXT NOT NULL DEFAULT '',
  progress_bars_done INTEGER NOT NULL DEFAULT 0,
  progress_bars_total INTEGER NOT NULL DEFAULT 0,
  progress_stage INTEGER NOT NULL DEFAULT 0,
  progress_stages INTEGER NOT NULL DEFAULT 0,
  progress_stage_label TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS backtest_kpi (
  backtest_id TEXT NOT NULL,
  symbol TEXT NOT NULL,
  bar_open INTEGER NOT NULL,
  algo TEXT NOT NULL,
  body TEXT NOT NULL,
  origin TEXT NOT NULL,
  spec TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (backtest_id, symbol, bar_open, algo)
);
`;

function catalogPath(): string {
  return path.join(resolveDataDir(), "backtests.sqlite");
}

function migrateCatalogColumns(db: Database.Database): void {
  const cols = db.prepare(`PRAGMA table_info(backtest)`).all() as { name: string }[];
  const names = new Set(cols.map((c) => c.name));
  if (!names.has("progress_stage")) {
    db.exec(`ALTER TABLE backtest ADD COLUMN progress_stage INTEGER NOT NULL DEFAULT 0`);
  }
  if (!names.has("progress_stages")) {
    db.exec(`ALTER TABLE backtest ADD COLUMN progress_stages INTEGER NOT NULL DEFAULT 0`);
  }
  if (!names.has("progress_stage_label")) {
    db.exec(`ALTER TABLE backtest ADD COLUMN progress_stage_label TEXT NOT NULL DEFAULT ''`);
  }
}

function openCatalog(): Database.Database {
  const dir = resolveDataDir();
  fs.mkdirSync(path.join(dir, "backtests"), { recursive: true });
  const db = new Database(catalogPath());
  db.pragma("busy_timeout = 5000");
  db.exec(SCHEMA);
  migrateCatalogColumns(db);
  return db;
}

export { defaultBacktestSettings };

function rowToRecord(row: Record<string, unknown>): BacktestRecord {
  const settings = JSON.parse(String(row.settings_json)) as BacktestSettings;
  return {
    id: String(row.id),
    name: String(row.name),
    description: String(row.description ?? ""),
    status: String(row.status),
    settings,
    workspaceRelPath: String(row.workspace_rel_path),
    createdAt: Number(row.created_at),
    startedAt: row.started_at != null ? Number(row.started_at) : null,
    finishedAt: row.finished_at != null ? Number(row.finished_at) : null,
    error: String(row.error ?? ""),
    progressDone: Number(row.progress_bars_done ?? 0),
    progressTotal: Number(row.progress_bars_total ?? 0),
    progressStage: Number(row.progress_stage ?? 0),
    progressStages: Number(row.progress_stages ?? 0),
    progressStageLabel: String(row.progress_stage_label ?? ""),
  };
}

export function listBacktests(): BacktestRecord[] {
  const db = openCatalog();
  try {
    const rows = db
      .prepare(
        `SELECT id, name, description, status, settings_json, workspace_rel_path, created_at,
                started_at, finished_at, error, progress_bars_done, progress_bars_total,
                progress_stage, progress_stages, progress_stage_label
         FROM backtest ORDER BY created_at DESC`,
      )
      .all() as Record<string, unknown>[];
    return rows.map(rowToRecord);
  } finally {
    db.close();
  }
}

export function getBacktest(id: string): BacktestRecord | null {
  const db = openCatalog();
  try {
    const row = db
      .prepare(
        `SELECT id, name, description, status, settings_json, workspace_rel_path, created_at,
                started_at, finished_at, error, progress_bars_done, progress_bars_total,
                progress_stage, progress_stages, progress_stage_label
         FROM backtest WHERE id = ?`,
      )
      .get(id) as Record<string, unknown> | undefined;
    return row ? rowToRecord(row) : null;
  } finally {
    db.close();
  }
}

/** Defaults for the UI and create API. Lake end caps `toMs` when KPIs exist; ingest fills gaps on run. */
export function resolveBacktestSettings(settings?: Partial<BacktestSettings>): BacktestSettings {
  const merged = { ...defaultBacktestSettings(), ...settings };
  const lake = lakeRangeForSymbol(merged.symbol);
  if (lake && merged.toMs > lake.toMs) {
    merged.toMs = lake.toMs;
  }
  return merged;
}

export function createBacktest(
  name: string,
  description: string,
  settings?: Partial<BacktestSettings>,
): BacktestRecord {
  const merged = resolveBacktestSettings(settings);
  const id = randomUUID();
  const rel = path.join("backtests", id);
  const created = Date.now();
  const db = openCatalog();
  try {
    db.prepare(
      `INSERT INTO backtest (id, name, description, status, settings_json, workspace_rel_path, created_at)
       VALUES (?, ?, ?, 'draft', ?, ?, ?)`,
    ).run(id, name.trim(), description, JSON.stringify(merged), rel, created);
    fs.mkdirSync(path.join(resolveDataDir(), rel), { recursive: true });
  } finally {
    db.close();
  }
  return getBacktest(id)!;
}

/** Mark catalog row running as soon as the run is accepted (before the Docker job starts). */
export function markBacktestRunning(id: string): BacktestRecord | null {
  const existing = getBacktest(id);
  if (!existing) {
    return null;
  }
  const now = Date.now();
  const db = openCatalog();
  try {
    db.prepare(
      `UPDATE backtest SET status = 'running', started_at = ?, finished_at = NULL, error = '',
       progress_bars_done = 0, progress_bars_total = 0,
       progress_stage = 1, progress_stages = 4, progress_stage_label = 'Starting' WHERE id = ?`,
    ).run(now, id);
  } finally {
    db.close();
  }
  return getBacktest(id);
}

export function updateBacktest(
  id: string,
  name: string,
  description: string,
  settings: BacktestSettings,
): BacktestRecord | null {
  const existing = getBacktest(id);
  if (!existing || existing.status === "running") {
    return null;
  }
  const db = openCatalog();
  try {
    db.prepare(
      `UPDATE backtest SET name = ?, description = ?, settings_json = ?, status = 'draft' WHERE id = ?`,
    ).run(name.trim(), description, JSON.stringify(settings), id);
  } finally {
    db.close();
  }
  return getBacktest(id);
}

export function backtestWorkspaceDir(rec: BacktestRecord): string {
  return path.join(resolveDataDir(), rec.workspaceRelPath);
}
