import type { MetricStats, RunReport, ExperimentInstance, ExperimentRecord } from "@/lib/experiment-shared";

export type AggregatedMetricStats = {
  avg: number;
  med: number;
  p95: number;
  n: number;
};

export type ExperimentRuntimeAggregate = {
  instanceCount: number;
  timedCount: number;
  /** Sum of per-instance durationMs (total sim compute wall). */
  totalDurationMs: number;
  /** Experiment catalog wall clock when both endpoints exist. */
  experimentWallMs: number | null;
  duration: AggregatedMetricStats | null;
  cpuPercent: AggregatedMetricStats | null;
  rssBytes: AggregatedMetricStats | null;
  heapAllocBytes: AggregatedMetricStats | null;
};

function percentileSorted(sorted: number[], p: number): number {
  if (sorted.length === 0) return NaN;
  if (sorted.length === 1) return sorted[0]!;
  const idx = p * (sorted.length - 1);
  const lo = Math.floor(idx);
  const hi = Math.ceil(idx);
  if (lo === hi) return sorted[lo]!;
  const w = idx - lo;
  return sorted[lo]! * (1 - w) + sorted[hi]! * w;
}

/** avg / med / p95 over samples; null when empty. */
export function statsFromSamples(samples: number[]): AggregatedMetricStats | null {
  const clean = samples.filter((v) => Number.isFinite(v));
  if (clean.length === 0) return null;
  const sum = clean.reduce((a, b) => a + b, 0);
  const sorted = [...clean].sort((a, b) => a - b);
  return {
    avg: sum / clean.length,
    med: percentileSorted(sorted, 0.5),
    p95: percentileSorted(sorted, 0.95),
    n: clean.length,
  };
}

function instanceDurationMs(inst: ExperimentInstance): number | null {
  if (inst.runReport && Number.isFinite(inst.runReport.durationMs)) {
    return inst.runReport.durationMs;
  }
  if (inst.metrics?.durationMs != null && Number.isFinite(inst.metrics.durationMs)) {
    return inst.metrics.durationMs;
  }
  if (inst.durationMs != null && Number.isFinite(inst.durationMs)) {
    return inst.durationMs;
  }
  if (inst.startedAtMs != null && inst.finishedAtMs != null) {
    return inst.finishedAtMs - inst.startedAtMs;
  }
  return null;
}

function pickInstanceAvg(stats: MetricStats | undefined): number | null {
  if (!stats || !Number.isFinite(stats.avg)) return null;
  return stats.avg;
}

/**
 * Aggregate instance run reports for the experiment detail Performance Metrics section.
 * Resource gauges use each instance's avg as one sample (then avg/med/p95 across instances).
 */
export function aggregateExperimentRuntime(
  experiment: Pick<ExperimentRecord, "startedAtMs" | "finishedAtMs" | "instances">,
): ExperimentRuntimeAggregate | null {
  const instances = experiment.instances ?? [];
  const durations: number[] = [];
  const cpu: number[] = [];
  const rss: number[] = [];
  const heap: number[] = [];

  for (const inst of instances) {
    const d = instanceDurationMs(inst);
    if (d != null && d >= 0) durations.push(d);
    const report: RunReport | null = inst.runReport;
    if (!report) continue;
    const c = pickInstanceAvg(report.cpuPercent);
    if (c != null) cpu.push(c);
    const r = pickInstanceAvg(report.rssBytes);
    if (r != null) rss.push(r);
    const h = pickInstanceAvg(report.heapAllocBytes);
    if (h != null) heap.push(h);
  }

  if (durations.length === 0 && cpu.length === 0 && rss.length === 0 && heap.length === 0) {
    return null;
  }

  let experimentWallMs: number | null = null;
  if (experiment.startedAtMs != null && experiment.finishedAtMs != null && experiment.finishedAtMs >= experiment.startedAtMs) {
    experimentWallMs = experiment.finishedAtMs - experiment.startedAtMs;
  }

  return {
    instanceCount: instances.length,
    timedCount: durations.length,
    totalDurationMs: durations.reduce((a, b) => a + b, 0),
    experimentWallMs,
    duration: statsFromSamples(durations),
    cpuPercent: statsFromSamples(cpu),
    rssBytes: statsFromSamples(rss),
    heapAllocBytes: statsFromSamples(heap),
  };
}

export function formatDurationMs(ms: number | null | undefined): string {
  if (ms == null || !Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  const sec = ms / 1000;
  if (sec < 60) return `${sec < 10 ? sec.toFixed(2) : sec.toFixed(1)} s`;
  const m = Math.floor(sec / 60);
  const s = sec - m * 60;
  return `${m}m ${s.toFixed(0)}s`;
}

export function formatCpuPercent(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return "—";
  return `${value.toFixed(1)}%`;
}

export function formatBytes(bytes: number | null | undefined): string {
  if (bytes == null || !Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb < 10 ? kb.toFixed(1) : Math.round(kb)} KB`;
  const mb = kb / 1024;
  if (mb < 1024) return `${mb < 10 ? mb.toFixed(2) : mb.toFixed(1)} MB`;
  const gb = mb / 1024;
  return `${gb.toFixed(2)} GB`;
}
