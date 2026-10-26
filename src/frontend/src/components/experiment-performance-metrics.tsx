
import {
  BacktestDetailSection,
  BacktestMetricTile,
} from "@/components/backtest-detail-section";
import {
  aggregateExperimentRuntime,
  formatBytes,
  formatCpuPercent,
  formatDurationMs,
  type AggregatedMetricStats,
  type ExperimentRuntimeAggregate,
} from "@/lib/experiment-run-metrics";
import type { ExperimentRecord } from "@/lib/experiment-shared";
import { cn } from "@/lib/utils";

const tileClass = "min-w-0 w-full";

function StatTriple({
  title,
  stats,
  format,
}: {
  title: string;
  stats: AggregatedMetricStats;
  format: (v: number) => string;
}) {
  return (
    <div className="min-w-0 space-y-1.5">
      <p className="text-xs font-medium text-muted-foreground">{title}</p>
      <div className="grid grid-cols-3 gap-2">
        <BacktestMetricTile className={tileClass} label="Avg" value={format(stats.avg)} />
        <BacktestMetricTile className={tileClass} label="Med" value={format(stats.med)} />
        <BacktestMetricTile className={tileClass} label="P95" value={format(stats.p95)} />
      </div>
    </div>
  );
}

function MetricsBody({ agg }: { agg: ExperimentRuntimeAggregate }) {
  const summaryCount = agg.experimentWallMs != null ? 3 : 2;
  const groups = [
    agg.duration && (
      <StatTriple
        key="duration"
        title="Instance duration"
        stats={agg.duration}
        format={formatDurationMs}
      />
    ),
    agg.cpuPercent && (
      <StatTriple key="cpu" title="CPU %" stats={agg.cpuPercent} format={formatCpuPercent} />
    ),
    agg.rssBytes && (
      <StatTriple key="rss" title="RSS" stats={agg.rssBytes} format={formatBytes} />
    ),
    agg.heapAllocBytes && (
      <StatTriple
        key="heap"
        title="Heap alloc"
        stats={agg.heapAllocBytes}
        format={formatBytes}
      />
    ),
  ].filter(Boolean);

  return (
    <div className="space-y-3">
      <div
        className={cn(
          "grid gap-2",
          summaryCount === 3 ? "grid-cols-3" : "grid-cols-2",
        )}
      >
        <BacktestMetricTile
          className={tileClass}
          label="Timed"
          value={`${agg.timedCount}/${agg.instanceCount}`}
          title="Instances with recorded sim duration"
        />
        <BacktestMetricTile
          className={tileClass}
          label="Total sim"
          value={formatDurationMs(agg.totalDurationMs)}
          title="Sum of instance simulation durations"
        />
        {agg.experimentWallMs != null && (
          <BacktestMetricTile
            className={tileClass}
            label="Experiment wall"
            value={formatDurationMs(agg.experimentWallMs)}
            title="Experiment started_at → finished_at"
          />
        )}
      </div>
      {groups.length > 0 && (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 sm:gap-x-6 sm:gap-y-5">
          {groups}
        </div>
      )}
    </div>
  );
}

export function ExperimentPerformanceMetrics({
  experiment,
  className,
}: {
  experiment: ExperimentRecord;
  className?: string;
}) {
  const agg = aggregateExperimentRuntime(experiment);
  if (!agg) return null;

  return (
    <BacktestDetailSection title="Performance Metrics" className={cn(className)}>
      <MetricsBody agg={agg} />
    </BacktestDetailSection>
  );
}
