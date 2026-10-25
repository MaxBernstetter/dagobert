"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import {
  formatStrategyKpiValue,
  type StrategyKpiComparison,
  type StrategyKpiResult,
} from "@/lib/strategy-kpi-shared";
import { BacktestPerformanceSummary } from "@/components/backtest-performance-summary";
import { cn } from "@/lib/utils";

type KpiSet = {
  results: StrategyKpiResult[];
  comparison?: StrategyKpiComparison;
};

export function StrategyKpiTags({
  playbook,
  symbol,
  fromMs = 0,
  toMs = 0,
  className,
  kpiApiUrl,
}: {
  playbook: string;
  symbol: string;
  fromMs?: number;
  toMs?: number;
  className?: string;
  kpiApiUrl?: string;
}) {
  const [results, setResults] = useState<StrategyKpiResult[] | null>(null);
  const [comparison, setComparison] = useState<StrategyKpiComparison | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        if (!kpiApiUrl) {
          if (!cancelled) {
            setResults([]);
            setComparison(undefined);
          }
          return;
        }
        const res = await fetch(kpiApiUrl);
        if (!res.ok) {
          if (!cancelled) {
            setResults([]);
            setComparison(undefined);
          }
          return;
        }
        const data = (await res.json()) as KpiSet;
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
  }, [kpiApiUrl]);

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
