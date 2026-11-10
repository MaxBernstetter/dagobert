import { fromJson, toJson, type DescMessage, type JsonObject, type MessageShape } from "@bufbuild/protobuf";
import { ConnectError, createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  CreateExperimentRequestSchema,
  ExperimentSchema,
  ExperimentProgressSchema,
} from "@/lib/gen/tradingcopilot/v1/experiment_pb";
import { ExperimentInstanceSchema } from "@/lib/gen/tradingcopilot/v1/experiment_instance_pb";
import type { ChartSeries } from "@/lib/gen/tradingcopilot/v1/experiment_chart_pb";
import {
  ExperimentService,
  KpiReportSchema,
  ListStrategiesResponseSchema,
} from "@/lib/gen/tradingcopilot/v1/experiment_service_pb";
import type { StrategyKpiSet } from "@/lib/strategy-kpi-shared";
import type { ActivityRow, JournalRow } from "@/types/read";

/**
 * The Experiment API: Connect-RPC with binary protobuf bodies, same origin unless
 * VITE_EXPERIMENT_API_URL names another host. Responses that the views read as plain JSON
 * (protojson shape: camelCase, int64 as strings) go through toJson.
 */
const client = createClient(
  ExperimentService,
  createConnectTransport({
    baseUrl: (import.meta.env.VITE_EXPERIMENT_API_URL ?? "").replace(/\/$/, ""),
    useBinaryFormat: true,
  }),
);

/** The server's message of a failed call. */
export function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ConnectError) return err.rawMessage;
  return err instanceof Error ? err.message : fallback;
}

function json<D extends DescMessage>(schema: D, msg: MessageShape<D>): JsonObject {
  return toJson(schema, msg, { alwaysEmitImplicit: true }) as JsonObject;
}

export async function listExperiments(): Promise<JsonObject[]> {
  const res = await client.listExperiments({});
  return res.experiments.map((e) => json(ExperimentSchema, e));
}

export async function getExperiment(id: string): Promise<JsonObject> {
  const res = await client.getExperiment({ id });
  return json(ExperimentSchema, res.experiment!);
}

export async function getExperimentProgress(id: string): Promise<JsonObject> {
  const res = await client.getExperimentProgress({ id });
  return json(ExperimentProgressSchema, res.progress!);
}

/** body is the protojson form of CreateExperimentRequest. */
export async function createExperiment(body: Record<string, unknown>): Promise<JsonObject> {
  const res = await client.createExperiment(fromJson(CreateExperimentRequestSchema, body as JsonObject));
  return json(ExperimentSchema, res.experiment!);
}

export async function runExperiment(id: string, retryFailed: boolean): Promise<void> {
  await client.runExperiment({ id, retryFailed });
}

export async function cancelExperiment(id: string): Promise<void> {
  await client.cancelExperiment({ id });
}

export async function deleteExperiment(id: string): Promise<void> {
  await client.deleteExperiment({ id });
}

export async function listStrategies(): Promise<JsonObject[]> {
  const res = await client.listStrategies({});
  return (json(ListStrategiesResponseSchema, res).strategies ?? []) as JsonObject[];
}

export async function getMaxParallelism(): Promise<number> {
  return (await client.getExperimentLimits({})).maxParallelism;
}

export type InstanceQuery = { limit: number; cursor: number; sort?: string; order?: string; q?: string };

export async function listInstances(
  experimentId: string,
  query: InstanceQuery,
): Promise<{ instances: JsonObject[]; total: number }> {
  const res = await client.listInstances({
    experimentId,
    limit: query.limit,
    cursor: String(query.cursor),
    sort: query.sort ?? "",
    order: query.order ?? "",
    q: query.q ?? "",
  });
  return {
    instances: res.instances.map((i) => json(ExperimentInstanceSchema, i)),
    total: Number(res.total),
  };
}

export async function getInstanceReport(experimentId: string, instanceId: string): Promise<JsonObject> {
  const res = await client.getInstanceReport({ experimentId, instanceId });
  return res.report!;
}

/** What a KPI view loads: the KPIs of one instance, or of the whole experiment without instanceId. */
export type KpiSource = { experimentId: string; instanceId?: string };

// The views read an unset statistic as null.
const NULLABLE_STATISTICS = [
  "monthlyReturnAvg",
  "monthlyReturnMed",
  "yearlyReturnAvg",
  "yearlyReturnMed",
  "tradesPerMonthAvg",
  "tradesPerMonthMed",
] as const;

export async function loadKpiSet(source: KpiSource): Promise<StrategyKpiSet> {
  const { experimentId, instanceId } = source;
  const report = instanceId
    ? (await client.getInstanceKpis({ experimentId, instanceId })).kpis
    : (await client.getExperimentPerformance({ id: experimentId })).performance;
  const set = json(KpiReportSchema, report!) as unknown as StrategyKpiSet;
  const stats = set.statistics as unknown as Record<string, unknown> | undefined;
  if (stats) for (const key of NULLABLE_STATISTICS) stats[key] ??= null;
  return set;
}

export type InstanceActivity = {
  rows: ActivityRow[];
  journals: JournalRow[];
  playbookTitles: Record<string, string>;
};

/** Experiment instances record orders and fills only: the signal and action fields of a row stay null. */
export async function getInstanceActivity(experimentId: string, instanceId: string): Promise<InstanceActivity> {
  const res = await client.getInstanceActivity({ experimentId, instanceId });
  return {
    rows: res.rows.map((r) => ({
      orderLinkId: r.orderLinkId,
      playbook: r.playbook,
      symbol: r.symbol,
      candleClose: Number(r.candleClose),
      kind: r.kind as ActivityRow["kind"],
      signalDecision: null,
      signalRule: null,
      decision: r.decision ?? null,
      rule: null,
      size: null,
      exit: null,
      writtenAt: null,
      orderId: r.orderId ?? null,
      orderStatus: r.orderStatus ?? null,
      venueSymbol: r.venueSymbol ?? null,
      side: r.side ?? null,
      qty: r.qty ?? null,
      cumExecQty: r.cumExecQty ?? null,
      cumExecValue: r.cumExecValue ?? null,
      avgPrice: r.avgPrice ?? null,
      marketUnit: r.marketUnit ?? null,
      retCode: r.retCode ?? null,
      retMsg: r.retMsg ?? null,
    })),
    journals: res.journals.map((j) => ({
      orderLinkId: j.orderLinkId,
      venue: j.venue,
      venueSymbol: j.venueSymbol,
      side: j.side,
      baseQty: j.baseQty,
      quoteQty: j.quoteQty,
      price: j.price,
      fee: j.fee,
      feeCurrency: j.feeCurrency,
      priceSource: j.priceSource,
    })),
    playbookTitles: res.playbookTitles,
  };
}

export type ChartSeriesQuery = {
  experimentId: string;
  instanceId: string;
  fromSec: number;
  toSec: number;
  maxPoints?: number;
};

export async function getChartSeries(q: ChartSeriesQuery): Promise<ChartSeries> {
  const res = await client.getChartSeries({
    experimentId: q.experimentId,
    instanceId: q.instanceId,
    fromSec: BigInt(q.fromSec),
    toSec: BigInt(q.toSec),
    maxPoints: q.maxPoints ?? 0,
  });
  return res.series!;
}

