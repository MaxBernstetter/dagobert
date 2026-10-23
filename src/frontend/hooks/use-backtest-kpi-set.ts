"use client";

import { useEffect, useState } from "react";
import type {
  BacktestRunStatistics,
  StrategyKpiComparison,
  StrategyKpiResult,
  StrategyKpiSet,
} from "@/lib/strategy-kpi-shared";

export type BacktestKpiSetState = {
  loading: boolean;
  results: StrategyKpiResult[];
  comparison: StrategyKpiComparison | undefined;
  statistics: BacktestRunStatistics | undefined;
};

export function useBacktestKpiSet(kpiApiUrl: string | undefined): BacktestKpiSetState {
  const [loading, setLoading] = useState(Boolean(kpiApiUrl));
  const [results, setResults] = useState<StrategyKpiResult[]>([]);
  const [comparison, setComparison] = useState<StrategyKpiComparison | undefined>(undefined);
  const [statistics, setStatistics] = useState<BacktestRunStatistics | undefined>(undefined);

  useEffect(() => {
    if (!kpiApiUrl) {
      setLoading(false);
      setResults([]);
      setComparison(undefined);
      setStatistics(undefined);
      return;
    }
    let cancelled = false;
    setLoading(true);
    (async () => {
      try {
        const res = await fetch(kpiApiUrl, { cache: "no-store" });
        if (!res.ok) {
          if (!cancelled) {
            setResults([]);
            setComparison(undefined);
            setStatistics(undefined);
            setLoading(false);
          }
          return;
        }
        const data = (await res.json()) as StrategyKpiSet;
        if (!cancelled) {
          setResults(data.results ?? []);
          setComparison(data.comparison);
          setStatistics(data.statistics);
          setLoading(false);
        }
      } catch {
        if (!cancelled) {
          setResults([]);
          setComparison(undefined);
          setStatistics(undefined);
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [kpiApiUrl]);

  return { loading, results, comparison, statistics };
}
