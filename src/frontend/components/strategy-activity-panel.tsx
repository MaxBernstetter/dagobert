"use client";

import { useEffect, useState } from "react";
import { ActivityView } from "@/components/activity-view";
import type { ActivityRow, JournalRow, LiveState } from "@/lib/read";
import { cn } from "@/lib/utils";

export function StrategyActivityPanel({
  live,
  className,
}: {
  live: LiveState;
  className?: string;
}) {
  const [rows, setRows] = useState<ActivityRow[] | null>(null);
  const [journals, setJournals] = useState<JournalRow[]>([]);
  const [playbookTitles, setPlaybookTitles] = useState<Record<string, string>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await fetch("/api/activity", { cache: "no-store" });
        if (!res.ok) {
          throw new Error(await res.text());
        }
        const data = (await res.json()) as {
          rows: ActivityRow[];
          journals?: JournalRow[];
          playbookTitles?: Record<string, string>;
        };
        if (!cancelled) {
          setError("");
          setRows(data.rows);
          setJournals(data.journals ?? []);
          setPlaybookTitles(data.playbookTitles ?? {});
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load activity");
          setRows([]);
        }
      }
    }

    void load();
    const timer = window.setInterval(() => {
      if (!cancelled) void load();
    }, 10_000);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);

  return (
    <div className={cn("flex min-h-0 flex-col gap-3", className)}>
      {error && <p className="shrink-0 text-sm text-destructive">{error}</p>}
      {rows === null && !error && (
        <p className="text-sm text-muted-foreground py-6 text-center">Loading decisions…</p>
      )}
      {rows !== null && (
        <ActivityView
          rows={rows}
          journals={journals}
          playbookTitles={playbookTitles}
          live={live}
          className="min-h-0 flex-1"
        />
      )}
    </div>
  );
}
