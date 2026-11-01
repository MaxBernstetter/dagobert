/**
 * App wire types + helpers for tradingcopilot.v1 (REST protojson).
 * Generated Message schemas live in @/lib/gen/tradingcopilot/v1 — re-exported below.
 * Wire shapes use number for int64 (JSON.parse), matching Go protojson output.
 */
import { create } from "@bufbuild/protobuf";
import { DEFAULT_MARKET_PROVIDER } from "@/lib/market-providers";
import {
  ExperimentCreateMode as ProtoExperimentCreateMode,
  ExperimentSettingsSchema,
  type ExperimentSettings as GenExperimentSettings,
} from "@/lib/gen/tradingcopilot/v1";

export {
  ExperimentSchema,
  ExperimentSettingsSchema,
  ExperimentProgressSchema,
  ExperimentInstanceSchema,
  InstanceMetricsSchema,
  ListExperimentsResponseSchema,
  ListInstancesResponseSchema,
  CreateExperimentRequestSchema,
  ExperimentGridSpecSchema,
  ExperimentInstanceSpecSchema,
  GridAxisSchema,
  MaCrossConfigSchema,
  SignalConsensusConfigSchema,
  RunExperimentRequestSchema,
  RunExperimentResponseSchema,
  OkResponseSchema,
  ErrorResponseSchema,
  ExperimentWorkerBatchSchema,
  ExperimentSFNInputSchema,
  ExperimentCreateMode as ProtoExperimentCreateMode,
} from "@/lib/gen/tradingcopilot/v1";

export type {
  Experiment,
  ExperimentSettings as GenExperimentSettings,
  ExperimentInstance as GenExperimentInstance,
  InstanceMetrics as GenInstanceMetrics,
  ListExperimentsResponse,
  ListInstancesResponse,
  CreateExperimentRequest,
  ExperimentGridSpec as GenExperimentGridSpec,
  ExperimentInstanceSpec as GenExperimentInstanceSpec,
  GridAxis as GenGridAxis,
  RunExperimentRequest,
  RunExperimentResponse,
  OkResponse,
  ErrorResponse,
  ExperimentWorkerBatch,
  ExperimentSFNInput,
} from "@/lib/gen/tradingcopilot/v1";

/** REST protojson ExperimentSettings (numbers, no $typeName required). */
export type ExperimentSettings = {
  provider: string;
  base: string;
  quote: string;
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
  /** C++ sweep processes; omitted = auto (one per CPU of the server). */
  parallelism?: number;
};

export type MaCrossConfig = {
  fastHr: number;
  slowHr: number;
};

export type SignalConsensusConfig = {
  rsiBull: number;
  rsiBear: number;
  volRatioBull: number;
  volRatioBear: number;
  atrStopMult: number;
  minBarsFlatAfterSell: number;
  minBarsLongBeforeSell: number;
};

export type StrategyConfig = MaCrossConfig | SignalConsensusConfig | Record<string, unknown>;

export type ExperimentInstanceSpec = {
  label?: string;
  playbook?: string;
  symbol?: string;
  maCross?: MaCrossConfig;
  signalConsensus?: SignalConsensusConfig;
  /** Catalog / legacy create path. */
  strategyConfig?: StrategyConfig;
};

export type MetricStats = {
  avg: number;
  med: number;
  p95: number;
};

export type RunReport = {
  startedAtMs: number;
  finishedAtMs: number;
  durationMs: number;
  cpuPercent?: MetricStats;
  rssBytes?: MetricStats;
  heapAllocBytes?: MetricStats;
};

export type AvgMed = {
  average: number;
  median: number;
};

export type PeriodReturn = {
  period: string;
  return: number;
};

export type InstanceRunStatistics = {
  durationMonths: number;
  durationYears: number;
  monthlyReturn?: AvgMed | null;
  yearlyReturn?: AvgMed | null;
  tradesPerMonth?: AvgMed | null;
  monthlyReturns?: PeriodReturn[];
  yearlyReturns?: PeriodReturn[];
};

export type InstanceReport = {
  version: number;
  run: RunReport;
  kpis: {
    playbook: string;
    symbol: string;
    fromMs: number;
    toMs: number;
    results: Array<{
      def: { slug: string; title?: string; label?: string; unit?: string; format?: string; kind?: string };
      value: number;
      extra?: Record<string, number>;
    }>;
    comparison?: {
      startCash: number;
      strategyReturn: number;
      buyHoldReturn: number;
    };
  };
  statistics: InstanceRunStatistics;
};

/** Denormalized table KPIs (nullable until instance completes). */
export type ExperimentInstanceMetrics = {
  durationMs: number | null;
  cagr: number | null;
  maxDrawdown: number | null;
  sharpe: number | null;
  sortino: number | null;
  monthlyReturnAvg: number | null;
  monthlyReturnMed: number | null;
  yearlyReturnAvg: number | null;
  yearlyReturnMed: number | null;
  tradesPerMonthAvg: number | null;
  tradesPerMonthMed: number | null;
  winRate: number | null;
  monthlyWinRateAvg: number | null;
  monthlyWinRateMed: number | null;
  yearlyWinRateAvg: number | null;
  yearlyWinRateMed: number | null;
};

export type ExperimentInstance = {
  id: string;
  experimentId: string;
  label: string;
  playbook: string;
  symbol: string;
  configHash: string;
  playbookKey: string;
  workspaceRelPath: string;
  status: string;
  error: string;
  startedAtMs: number | null;
  finishedAtMs: number | null;
  metrics: ExperimentInstanceMetrics;
  maCross?: MaCrossConfig;
  signalConsensus?: SignalConsensusConfig;
  /** Derived for UI / catalog fallback. */
  strategyConfig: StrategyConfig;
  durationMs: number | null;
  runReport: RunReport | null;
  instanceReport: InstanceReport | null;
};

/** UI/API experiment record matching tradingcopilot.v1.Experiment protojson (+ nested settings). */
export type ExperimentRecord = {
  id: string;
  name: string;
  description: string;
  status: string;
  error: string;
  provider: string;
  base: string;
  quote: string;
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
  createdAtMs: number;
  startedAtMs: number | null;
  finishedAtMs: number | null;
  progressStage: number;
  progressStages: number;
  progressStageLabel: string;
  progressInstanceDone: number;
  progressInstanceTotal: number;
  settings: ExperimentSettings;
  instances: ExperimentInstance[];
  listPlaybooks?: string[];
  listSymbols?: string[];
};

export type ExperimentProgress = {
  id: string;
  status: string;
  error: string;
  progressStage: number;
  progressStages: number;
  progressStageLabel: string;
  progressInstanceDone: number;
  progressInstanceTotal: number;
  failedInstanceCount: number;
  startedAtMs: number | null;
  finishedAtMs: number | null;
};

export type ExperimentInstancePage = {
  instances: ExperimentInstance[];
  total: number;
  nextCursor: string | null;
  limit: number;
};

export type ExperimentGridSpec = {
  playbook: string;
  axes: Array<{ key: string; values?: string[]; numericValues?: number[] }>;
};

function num(v: unknown, fallback = 0): number {
  if (v == null) return fallback;
  if (typeof v === "bigint") return Number(v);
  const n = Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function numOrNull(v: unknown): number | null {
  if (v == null) return null;
  if (typeof v === "bigint") return Number(v);
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

export function settingsOf(s: Partial<ExperimentRecord> | ExperimentSettings | null | undefined): ExperimentSettings {
  if (!s) return defaultExperimentSettings();
  if ("settings" in s && s.settings) return { ...defaultExperimentSettings(), ...s.settings };
  return {
    provider: String((s as ExperimentSettings).provider ?? DEFAULT_MARKET_PROVIDER),
    base: String((s as ExperimentSettings).base ?? "BTC"),
    quote: String((s as ExperimentSettings).quote ?? "USDT"),
    fromMs: num((s as ExperimentSettings).fromMs),
    toMs: num((s as ExperimentSettings).toMs),
    initialUsdc: num((s as ExperimentSettings).initialUsdc, 100_000),
    initialUsdt: num((s as ExperimentSettings).initialUsdt),
    initialBase: num((s as ExperimentSettings).initialBase),
    targetWeightPct: num((s as ExperimentSettings).targetWeightPct, 100),
    cashReserveQuote: num((s as ExperimentSettings).cashReserveQuote, 0),
    minOrderQuote: num((s as ExperimentSettings).minOrderQuote, 1),
    redownloadMissing: Boolean((s as ExperimentSettings).redownloadMissing),
    algo: String((s as ExperimentSettings).algo ?? "v1"),
    spreadBps: num((s as ExperimentSettings).spreadBps, 4),
    slippageBps: num((s as ExperimentSettings).slippageBps, 2),
    feeBps: num((s as ExperimentSettings).feeBps, 6),
  };
}

export function strategyConfigOf(inst: {
  playbook?: string;
  maCross?: MaCrossConfig;
  signalConsensus?: SignalConsensusConfig;
  strategyConfig?: StrategyConfig;
}): StrategyConfig {
  if (inst.strategyConfig && Object.keys(inst.strategyConfig).length > 0) return inst.strategyConfig;
  if (inst.maCross) return inst.maCross;
  if (inst.signalConsensus) return inst.signalConsensus;
  return {};
}

/** Normalize a REST protojson Experiment (or legacy catalog row) into ExperimentRecord. */
export function normalizeExperiment(raw: Record<string, unknown>): ExperimentRecord {
  const flat = raw as Partial<ExperimentRecord> & Record<string, unknown>;
  const settings = settingsOf(
    (flat.settings as ExperimentSettings | undefined) ??
      ({
        provider: flat.provider,
        base: flat.base,
        quote: flat.quote,
        fromMs: flat.fromMs,
        toMs: flat.toMs,
        initialUsdc: flat.initialUsdc,
        initialUsdt: flat.initialUsdt,
        initialBase: flat.initialBase,
        targetWeightPct: flat.targetWeightPct,
        cashReserveQuote: flat.cashReserveQuote,
        minOrderQuote: flat.minOrderQuote,
        redownloadMissing: flat.redownloadMissing,
        algo: flat.algo,
        spreadBps: flat.spreadBps,
        slippageBps: flat.slippageBps,
        feeBps: flat.feeBps,
      } as ExperimentSettings),
  );
  const startedAtMs = numOrNull(flat.startedAtMs ?? (flat as {startedAt?: unknown}).startedAt);
  const finishedAtMs = numOrNull(flat.finishedAtMs ?? (flat as {finishedAt?: unknown}).finishedAt);
  return {
    id: String(flat.id ?? ""),
    name: String(flat.name ?? ""),
    description: String(flat.description ?? ""),
    status: String(flat.status ?? ""),
    error: String(flat.error ?? ""),
    provider: settings.provider,
    base: settings.base,
    quote: settings.quote,
    fromMs: settings.fromMs,
    toMs: settings.toMs,
    initialUsdc: settings.initialUsdc,
    initialUsdt: settings.initialUsdt,
    initialBase: settings.initialBase,
    targetWeightPct: settings.targetWeightPct,
    cashReserveQuote: settings.cashReserveQuote,
    minOrderQuote: settings.minOrderQuote,
    redownloadMissing: settings.redownloadMissing,
    algo: settings.algo,
    spreadBps: settings.spreadBps,
    slippageBps: settings.slippageBps,
    feeBps: settings.feeBps,
    createdAtMs: num(flat.createdAtMs ?? (flat as {createdAt?: unknown}).createdAt),
    startedAtMs,
    finishedAtMs,
    progressStage: num(flat.progressStage),
    progressStages: num(flat.progressStages),
    progressStageLabel: String(flat.progressStageLabel ?? ""),
    progressInstanceDone: num(flat.progressInstanceDone),
    progressInstanceTotal: num(flat.progressInstanceTotal),
    settings,
    instances: Array.isArray(flat.instances)
      ? (flat.instances as Record<string, unknown>[]).map(normalizeInstance)
      : [],
    listPlaybooks: Array.isArray(flat.listPlaybooks) ? (flat.listPlaybooks as string[]) : undefined,
    listSymbols: Array.isArray(flat.listSymbols) ? (flat.listSymbols as string[]) : undefined,
  };
}

export function normalizeInstance(raw: Record<string, unknown>): ExperimentInstance {
  const m = (raw.metrics ?? {}) as Record<string, unknown>;
  const metrics: ExperimentInstanceMetrics = {
    durationMs: numOrNull(m.durationMs),
    cagr: numOrNull(m.cagr),
    maxDrawdown: numOrNull(m.maxDrawdown),
    sharpe: numOrNull(m.sharpe),
    sortino: numOrNull(m.sortino),
    monthlyReturnAvg: numOrNull(m.monthlyReturnAvg),
    monthlyReturnMed: numOrNull(m.monthlyReturnMed),
    yearlyReturnAvg: numOrNull(m.yearlyReturnAvg),
    yearlyReturnMed: numOrNull(m.yearlyReturnMed),
    tradesPerMonthAvg: numOrNull(m.tradesPerMonthAvg),
    tradesPerMonthMed: numOrNull(m.tradesPerMonthMed),
    winRate: numOrNull(m.winRate),
    monthlyWinRateAvg: numOrNull(m.monthlyWinRateAvg),
    monthlyWinRateMed: numOrNull(m.monthlyWinRateMed),
    yearlyWinRateAvg: numOrNull(m.yearlyWinRateAvg),
    yearlyWinRateMed: numOrNull(m.yearlyWinRateMed),
  };
  const startedAtMs = numOrNull(raw.startedAtMs ?? raw.startedAt);
  const finishedAtMs = numOrNull(raw.finishedAtMs ?? raw.finishedAt);
  const maCross = raw.maCross as MaCrossConfig | undefined;
  const signalConsensus = raw.signalConsensus as SignalConsensusConfig | undefined;
  const strategyConfig = strategyConfigOf({
    maCross,
    signalConsensus,
    strategyConfig: raw.strategyConfig as StrategyConfig | undefined,
  });
  let durationMs = metrics.durationMs;
  if (durationMs == null && startedAtMs != null && finishedAtMs != null) {
    durationMs = finishedAtMs - startedAtMs;
  }
  return {
    id: String(raw.id ?? ""),
    experimentId: String(raw.experimentId ?? ""),
    label: String(raw.label ?? ""),
    playbook: String(raw.playbook ?? ""),
    symbol: String(raw.symbol ?? ""),
    configHash: String(raw.configHash ?? ""),
    playbookKey: String(raw.playbookKey ?? ""),
    workspaceRelPath: String(raw.workspaceRelPath ?? ""),
    status: String(raw.status ?? ""),
    error: String(raw.error ?? ""),
    startedAtMs,
    finishedAtMs,
    metrics,
    maCross,
    signalConsensus,
    strategyConfig,
    durationMs,
    runReport: (raw.runReport as RunReport | null) ?? null,
    instanceReport: (raw.instanceReport as InstanceReport | null) ?? null,
  };
}

export function normalizeProgress(raw: Record<string, unknown>): ExperimentProgress {
  return {
    id: String(raw.id ?? ""),
    status: String(raw.status ?? ""),
    error: String(raw.error ?? ""),
    progressStage: num(raw.progressStage),
    progressStages: num(raw.progressStages),
    progressStageLabel: String(raw.progressStageLabel ?? ""),
    progressInstanceDone: num(raw.progressInstanceDone),
    progressInstanceTotal: num(raw.progressInstanceTotal),
    failedInstanceCount: num(raw.failedInstanceCount),
    startedAtMs: numOrNull(raw.startedAtMs ?? raw.startedAt),
    finishedAtMs: numOrNull(raw.finishedAtMs ?? raw.finishedAt),
  };
}

export function defaultExperimentSettings(): ExperimentSettings {
  const now = Date.now();
  const to = now - (now % (60 * 60 * 1000));
  const from = to - 365 * 24 * 60 * 60 * 1000;
  const msg = create(ExperimentSettingsSchema, {
    provider: DEFAULT_MARKET_PROVIDER,
    base: "BTC",
    quote: "USDT",
    fromMs: BigInt(from),
    toMs: BigInt(to),
    initialUsdc: 100_000,
    initialUsdt: 0,
    initialBase: 0,
    targetWeightPct: 100,
    cashReserveQuote: 0,
    minOrderQuote: 1,
    redownloadMissing: false,
    algo: "v1",
    spreadBps: 4,
    slippageBps: 2,
    feeBps: 6,
  }) as GenExperimentSettings;
  return {
    provider: msg.provider,
    base: msg.base,
    quote: msg.quote,
    fromMs: Number(msg.fromMs),
    toMs: Number(msg.toMs),
    initialUsdc: msg.initialUsdc,
    initialUsdt: msg.initialUsdt,
    initialBase: msg.initialBase,
    targetWeightPct: msg.targetWeightPct,
    cashReserveQuote: msg.cashReserveQuote,
    minOrderQuote: msg.minOrderQuote,
    redownloadMissing: msg.redownloadMissing,
    algo: msg.algo,
    spreadBps: msg.spreadBps,
    slippageBps: msg.slippageBps,
    feeBps: msg.feeBps,
  };
}

export function experimentPipelineSymbol(settings: Pick<ExperimentSettings, "base" | "quote">): string {
  return `${(settings.base || "BTC").toUpperCase()}${(settings.quote || "USDT").toUpperCase()}`;
}

export function experimentProgressPercent(rec: {
  status: string;
  progressStage: number;
  progressStages: number;
  progressInstanceDone: number;
  progressInstanceTotal: number;
}): number | null {
  if (rec.status === "materializing") {
    if (rec.progressInstanceTotal <= 0) return 0;
    return Math.min(
      99,
      Math.round((100 * rec.progressInstanceDone) / rec.progressInstanceTotal),
    );
  }
  if (rec.status !== "running") return null;
  // Only the simulation stage counts instances, so its share is the only measured progress.
  // Before the instance total is known, no percentage is shown; the stage label stays visible.
  if (rec.progressInstanceTotal <= 0) return null;
  return Math.min(
    100,
    Math.round((100 * rec.progressInstanceDone) / rec.progressInstanceTotal),
  );
}

/** Status text for the table and the detail header. Uses the same percent and counts as the detail text. */
export function experimentStatusLabel(rec: ExperimentRecord): string {
  const pct = experimentProgressPercent(rec);
  if (rec.status === "materializing") {
    return pct == null ? "Materializing" : `Materializing · ${pct}%`;
  }
  if (rec.status !== "running") {
    return rec.status.charAt(0).toUpperCase() + rec.status.slice(1);
  }
  const parts = ["Running"];
  if (pct != null) parts.push(`${pct}%`);
  if (rec.progressInstanceTotal > 0) {
    parts.push(`${rec.progressInstanceDone}/${rec.progressInstanceTotal}`);
  }
  const stage = rec.progressStageLabel.trim();
  if (stage) parts.push(stage);
  return parts.join(" · ");
}

export function emptyExperimentInstanceMetrics(): ExperimentInstanceMetrics {
  return {
    durationMs: null,
    cagr: null,
    maxDrawdown: null,
    sharpe: null,
    sortino: null,
    monthlyReturnAvg: null,
    monthlyReturnMed: null,
    yearlyReturnAvg: null,
    yearlyReturnMed: null,
    tradesPerMonthAvg: null,
    tradesPerMonthMed: null,
    winRate: null,
    monthlyWinRateAvg: null,
    monthlyWinRateMed: null,
    yearlyWinRateAvg: null,
    yearlyWinRateMed: null,
  };
}

/** Map UI create mode to proto enum name accepted by protojson. */
export type ExperimentCreateMode = "grid" | "manual";

export function experimentCreateModeProto(mode: ExperimentCreateMode): ProtoExperimentCreateMode {
  return mode === "grid" ? ProtoExperimentCreateMode.GRID : ProtoExperimentCreateMode.MANUAL;
}

/** Convert experiment-grid axes to proto GridAxis (numericValues / values). */
export function toProtoGridAxes(
  axes: Array<{ key: string; values: Array<number | string> }>,
): ExperimentGridSpec["axes"] {
  return axes.map((a) => {
    const numericValues: number[] = [];
    const values: string[] = [];
    for (const v of a.values) {
      if (typeof v === "number") numericValues.push(v);
      else values.push(String(v));
    }
    return { key: a.key, numericValues, values };
  });
}
