
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import {
  formatStrategyKpiValue,
  type StrategyKpiComparison,
  type StrategyKpiResult,
} from "@/lib/strategy-kpi-shared";
import { BacktestPerformanceSummary } from "@/components/backtest-performance-summary";
import { loadKpiSet, type KpiSource } from "@/lib/experiment-api";
import { cn } from "@/lib/utils";

export function StrategyKpiTags({
  playbook,
  symbol,
  fromMs = 0,
  toMs = 0,
  className,
  kpiSource,
}: {
  playbook: string;
  symbol: string;
  fromMs?: number;
  toMs?: number;
  className?: string;
  kpiSource?: KpiSource;
}) {
  const [results, setResults] = useState<StrategyKpiResult[] | null>(null);
  const [comparison, setComparison] = useState<StrategyKpiComparison | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        if (!kpiSource) {
          if (!cancelled) {
            setResults([]);
            setComparison(undefined);
          }
          return;
        }
        const data = await loadKpiSet(kpiSource);
        if (!cancelled) {
          setResults(data.results ?? []);
          setComparison(data.comparison);
        }
      } catch {
        if (!cancelled) {
          setResults([]);
          setComparison(undefined);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the ids identify the source
  }, [kpiSource?.experimentId, kpiSource?.instanceId]);

  if (results === null) {
    return (
      <span className={cn("text-xs text-muted-foreground", className)} aria-hidden>
        · KPIs…
      </span>
    );
  }

  if (results.length === 0) {
    return null;
  }

  return (
    <div className={cn("space-y-2", className)}>
      <BacktestPerformanceSummary comparison={comparison} />
      <span className="inline-flex flex-wrap items-center gap-1.5">
      {results.map((r) => {
        const tip = r.def.title;
        const formatted = formatStrategyKpiValue(r.def.format, r.value);
        return (
          <Badge
            key={r.def.slug}
            variant="secondary"
            className="font-normal tabular-nums"
            title={tip}
          >
            <span className="text-muted-foreground">{r.def.label}</span>
            <span className="ml-1 font-medium text-foreground">{formatted}</span>
          </Badge>
        );
      })}
      </span>
    </div>
  );
}
