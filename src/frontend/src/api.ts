/** Protojson field names from tradingcopilot.v1 (camelCase). */

export type ExperimentSettings = {
  provider?: string;
  base?: string;
  quote?: string;
  fromMs?: number | string;
  toMs?: number | string;
  initialUsdc?: number;
  initialUsdt?: number;
  initialBase?: number;
  targetWeightPct?: number;
  cashReserveQuote?: number;
  minOrderQuote?: number;
  redownloadMissing?: boolean;
  algo?: string;
  spreadBps?: number;
  slippageBps?: number;
  feeBps?: number;
};

export type Experiment = {
  id: string;
  name: string;
  description?: string;
  status: string;
  error?: string;
  provider?: string;
  base?: string;
  quote?: string;
  fromMs?: number | string;
  toMs?: number | string;
  initialUsdc?: number;
  createdAtMs?: number | string;
  progressStageLabel?: string;
  progressInstanceDone?: number;
  progressInstanceTotal?: number;
  listPlaybooks?: string[];
  listSymbols?: string[];
  settings?: ExperimentSettings;
};

export type InstanceMetrics = {
  durationMs?: number | string;
  cagr?: number;
  maxDrawdown?: number;
  sharpe?: number;
  sortino?: number;
};

export type ExperimentInstance = {
  id: string;
  experimentId?: string;
  label?: string;
  playbook?: string;
  symbol?: string;
  status?: string;
  error?: string;
  metrics?: InstanceMetrics;
};

export type InstancePage = {
  instances: ExperimentInstance[];
  total: number | string;
  nextCursor?: string;
  limit: number;
};

export type ChartBar = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  indicators?: Record<string, number | null>;
};

export type ChartIndicatorMeta = { id: string; label: string; color: string };

export type ChartSeries = {
  bars: ChartBar[];
  indicators: ChartIndicatorMeta[];
};

export type CreateExperimentBody = {
  name: string;
  description?: string;
  settings: ExperimentSettings;
  instances: Array<{
    label?: string;
    playbook: string;
    symbol: string;
    maCross: { fastHr: number; slowHr: number };
  }>;
};

const base = (import.meta.env.VITE_EXPERIMENT_API_URL ?? "").replace(/\/$/, "");

export function apiUrl(path: string): string {
  const p = path.startsWith("/") ? path : `/${path}`;
  return `${base}${p}`;
}

async function readError(res: Response): Promise<string> {
  const text = await res.text();
  try {
    const body = JSON.parse(text) as { error?: string };
    if (body.error) return body.error;
  } catch {
    /* plain text */
  }
  return text || res.statusText;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(apiUrl(path), {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!res.ok) throw new Error(await readError(res));
  return (await res.json()) as T;
}

export function listExperiments(): Promise<{ experiments: Experiment[] }> {
  return request("/api/v1/experiments");
}

export function getExperiment(id: string): Promise<Experiment> {
  return request(`/api/v1/experiments/${encodeURIComponent(id)}`);
}

export function createExperiment(body: CreateExperimentBody): Promise<Experiment> {
  return request("/api/v1/experiments", { method: "POST", body: JSON.stringify(body) });
}

export function listInstances(
  experimentId: string,
  query: { limit: number; cursor?: string },
): Promise<InstancePage> {
  const q = new URLSearchParams({ limit: String(query.limit) });
  if (query.cursor) q.set("cursor", query.cursor);
  return request(`/api/v1/experiments/${encodeURIComponent(experimentId)}/instances?${q}`);
}

export function chartSeries(
  experimentId: string,
  instanceId: string,
  fromSec: number,
  toSec: number,
): Promise<ChartSeries> {
  const q = new URLSearchParams({ from: String(fromSec), to: String(toSec) });
  return request(
    `/api/v1/experiments/${encodeURIComponent(experimentId)}/instances/${encodeURIComponent(instanceId)}/chart-series?${q}`,
  );
}

export function asNumber(v: number | string | undefined | null): number | null {
  if (v == null || v === "") return null;
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : null;
}
