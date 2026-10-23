import type { ActivityRow } from "@/lib/read";

export type BacktestSettings = {
  symbol: string;
  playbook: string;
  fromMs: number;
  toMs: number;
  initialUsdc: number;
  initialUsdt: number;
  initialBase: number;
  targetWeightPct: number;
  cashReserveQuote: number;
  minOrderQuote: number;
  redownloadMissing: boolean;
  algo: string;
  spreadBps: number;
  slippageBps: number;
  feeBps: number;
};

export type BacktestRecord = {
  id: string;
  name: string;
  description: string;
  status: string;
  settings: BacktestSettings;
  workspaceRelPath: string;
  createdAt: number;
  startedAt: number | null;
  finishedAt: number | null;
  error: string;
  progressDone: number;
  progressTotal: number;
  progressStage: number;
  progressStages: number;
  progressStageLabel: string;
};

export const BACKTEST_STAGE_TOTAL = 4;

export function defaultBacktestSettings(): BacktestSettings {
  const now = Date.now();
  const to = now - (now % (60 * 60 * 1000));
  const from = to - 365 * 24 * 60 * 60 * 1000;
  return {
    symbol: "BTCUSDT",
    playbook: "ma-cross",
    fromMs: from,
    toMs: to,
    initialUsdc: 100_000,
    initialUsdt: 0,
    initialBase: 0,
    targetWeightPct: 95,
    cashReserveQuote: 100,
    minOrderQuote: 1,
    redownloadMissing: false,
    algo: "v1",
    spreadBps: 4,
    slippageBps: 2,
    feeBps: 6,
  };
}

/** Starting quote cash for KPI replay (USDC sleeve for BTCUSDT / ETHUSDT). */
export function backtestStartCash(settings: BacktestSettings): number {
  return settings.initialUsdc + settings.initialUsdt;
}

export function normalizePlaybookSlug(playbook: string): string {
  return playbook.replace(/\.ya?ml$/i, "").trim();
}

export function playbookIdForKpi(playbook: string): string {
  const base = normalizePlaybookSlug(playbook);
  return `${base}.yaml`;
}

export type PlaybookOption = {
  id: string;
  label: string;
};

/** Overall run progress 0–100 while status is running. */
export function backtestProgressPercent(rec: {
  status: string;
  progressStage: number;
  progressStages: number;
  progressDone: number;
  progressTotal: number;
}): number | null {
  if (rec.status !== "running") return null;
  const stages = rec.progressStages > 0 ? rec.progressStages : BACKTEST_STAGE_TOTAL;
  const stage = rec.progressStage > 0 ? rec.progressStage : 1;
  const stageWeight = 100 / stages;
  const stageBase = (stage - 1) * stageWeight;
  if (rec.progressTotal > 0) {
    const within = Math.min(1, rec.progressDone / rec.progressTotal);
    return Math.min(100, Math.round(stageBase + within * stageWeight));
  }
  return Math.min(100, Math.round(stageBase + stageWeight * 0.15));
}

export function backtestStatusLabel(rec: {
  status: string;
  progressStage: number;
  progressStages: number;
  progressStageLabel: string;
  progressDone: number;
  progressTotal: number;
}): string {
  if (rec.status !== "running") {
    return rec.status.charAt(0).toUpperCase() + rec.status.slice(1);
  }
  const stages = rec.progressStages > 0 ? rec.progressStages : BACKTEST_STAGE_TOTAL;
  const stage = rec.progressStage > 0 ? rec.progressStage : 1;
  const label = rec.progressStageLabel.trim();
  const pct = backtestProgressPercent(rec);
  const head = label
    ? `Running (${stage}/${stages} · ${label})`
    : `Running (${stage}/${stages})`;
  if (pct != null) {
    return `${head} · ${pct}%`;
  }
  return head;
}

export type BacktestOutcome = {
  decisions: number;
  fills: number;
  rejections: number;
  awaiting: number;
};

export function summarizeBacktestOutcome(rows: ActivityRow[]): BacktestOutcome {
  let fills = 0;
  let rejections = 0;
  let awaiting = 0;
  for (const row of rows) {
    switch (row.kind) {
      case "fill":
        fills++;
        break;
      case "rejection":
        rejections++;
        break;
      case "not-yet-filled":
      case "pending":
        awaiting++;
        break;
      default:
        break;
    }
  }
  return { decisions: rows.length, fills, rejections, awaiting };
}

export function formatBacktestRange(fromMs: number, toMs: number): string {
  const f = new Date(fromMs).toISOString().slice(0, 16).replace("T", " ");
  const t = new Date(toMs).toISOString().slice(0, 16).replace("T", " ");
  return `${f} → ${t} UTC`;
}

const HOUR_MS = 60 * 60 * 1000;

/** UTC midnight on the calendar day of `ms` (date picker / draft range start). */
export function utcMidnightFromMs(ms: number): number {
  const d = new Date(ms);
  return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate());
}

export function alignMsToHour(ms: number): number {
  return ms - (ms % HOUR_MS);
}

function utcDateParts(ms: number): string {
  const d = new Date(ms);
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** `type="date"` value for range start (UTC midnight). */
export function backtestFromMsToDateInput(fromMs: number): string {
  if (!Number.isFinite(fromMs) || fromMs <= 0) return "";
  return utcDateParts(fromMs);
}

/** Inclusive UTC end calendar day for `toMs` (range end is exclusive / hour-aligned). */
export function backtestToMsToDateInput(toMs: number): string {
  if (!Number.isFinite(toMs) || toMs <= 0) return "";
  return utcDateParts(toMs - 1);
}

export function backtestDateInputToFromMs(date: string): number | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim());
  if (!m) return null;
  const ms = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 0, 0, 0, 0);
  return Number.isFinite(ms) ? ms : null;
}

/** Hour-aligned range end after inclusive UTC `date` (matches backtest job). */
export function backtestDateInputToToMs(date: string): number | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim());
  if (!m) return null;
  let end = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + 1, 0, 0, 0, 0);
  end -= end % HOUR_MS;
  return Number.isFinite(end) ? end : null;
}

/** Local `Date` for calendar UI (wall date matches UTC range day). */
export function backtestDateInputToPickerDate(date: string): Date | undefined {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim());
  if (!m) return undefined;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

export function backtestPickerDateToDateInput(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

export function formatBacktestDateLabel(dateInput: string): string {
  const fromMs = backtestDateInputToFromMs(dateInput);
  if (fromMs == null) return "Pick a date";
  return new Date(fromMs).toLocaleDateString(undefined, {
    timeZone: "UTC",
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
