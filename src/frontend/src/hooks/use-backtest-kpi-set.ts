
import { useEffect, useState } from "react";
import { loadKpiSet, type KpiSource } from "@/lib/experiment-api";
import type {
  BacktestRunStatistics,
  StrategyKpiComparison,
  StrategyKpiResult,
} from "@/lib/strategy-kpi-shared";

export type BacktestKpiSetState = {
  loading: boolean;
  results: StrategyKpiResult[];
  comparison: StrategyKpiComparison | undefined;
  statistics: BacktestRunStatistics | undefined;
};

export function useBacktestKpiSet(source: KpiSource | undefined): BacktestKpiSetState {
  const [loading, setLoading] = useState(Boolean(source));
  const [results, setResults] = useState<StrategyKpiResult[]>([]);
  const [comparison, setComparison] = useState<StrategyKpiComparison | undefined>(undefined);
  const [statistics, setStatistics] = useState<BacktestRunStatistics | undefined>(undefined);

  useEffect(() => {
    if (!source) {
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
        const data = await loadKpiSet(source);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the ids identify the source
  }, [source?.experimentId, source?.instanceId]);

  return { loading, results, comparison, statistics };
}
