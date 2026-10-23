"use client";

import { useEffect, useMemo, useState } from "react";
import type { BacktestRecord } from "@/lib/backtest-shared";
import type { StrategyKpiSet } from "@/lib/strategy-kpi-shared";

export function useBacktestListKpis(backtests: BacktestRecord[]): {
  kpiById: Record<string, StrategyKpiSet | null>;
  loadingIds: Set<string>;
} {
  const [kpiById, setKpiById] = useState<Record<string, StrategyKpiSet | null>>({});
  const [loadingIds, setLoadingIds] = useState<Set<string>>(() => new Set());

  const fingerprint = useMemo(
    () => backtests.map((b) => `${b.id}:${b.status}:${b.finishedAt ?? ""}`).join("|"),
    [backtests],
  );

  useEffect(() => {
    let cancelled = false;
    const completed = backtests.filter((b) => b.status === "completed");
    const idSet = new Set(completed.map((b) => b.id));

    setKpiById((prev) => {
      const next: Record<string, StrategyKpiSet | null> = {};
      for (const id of idSet) {
        if (prev[id]) next[id] = prev[id];
      }
      return next;
    });

    if (completed.length === 0) {
      setLoadingIds(new Set());
      return () => {
        cancelled = true;
      };
    }

    setLoadingIds(new Set(completed.map((b) => b.id)));

    for (const b of completed) {
      const q = new URLSearchParams({
        fromMs: String(b.settings.fromMs),
        toMs: String(b.settings.toMs),
      });
      void fetch(`/api/backtests/${b.id}/kpis?${q}`, { cache: "no-store" })
        .then(async (res) => (res.ok ? ((await res.json()) as StrategyKpiSet) : null))
        .then((data) => {
          if (cancelled) return;
          setKpiById((prev) => ({ ...prev, [b.id]: data }));
          setLoadingIds((prev) => {
            const next = new Set(prev);
            next.delete(b.id);
            return next;
          });
        })
        .catch(() => {
          if (cancelled) return;
          setKpiById((prev) => ({ ...prev, [b.id]: null }));
          setLoadingIds((prev) => {
            const next = new Set(prev);
            next.delete(b.id);
            return next;
          });
        });
    }

    return () => {
      cancelled = true;
    };
  }, [fingerprint, backtests]);

  return { kpiById, loadingIds };
}
