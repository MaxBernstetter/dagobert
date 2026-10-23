"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ActivityView } from "@/components/activity-view";
import { BacktestSettingsForm } from "@/components/backtest-settings-form";
import { BacktestStatusBadge } from "@/components/backtest-status-badge";
import { buildBacktestTableColumns } from "@/components/backtest-table-columns";
import { FilterableTable } from "@/components/filterable-table";
import { useBacktestListKpis } from "@/hooks/use-backtest-list-kpis";
import { BacktestPerformancePanel } from "@/components/backtest-performance-panel";
import { BacktestStrategyMetadata } from "@/components/backtest-strategy-metadata";
import { useBacktestKpiSet } from "@/hooks/use-backtest-kpi-set";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { ActivityRow, JournalRow, LiveState } from "@/lib/read";
import {
  backtestProgressPercent,
  defaultBacktestSettings,
  summarizeBacktestOutcome,
  type BacktestRecord,
  type BacktestSettings,
  type PlaybookOption,
} from "@/lib/backtest-shared";
import { cn } from "@/lib/utils";

async function fetchDraftSettings(symbol: string): Promise<BacktestSettings> {
  const res = await fetch(
    `/api/backtests/draft-settings?symbol=${encodeURIComponent(symbol)}`,
    { cache: "no-store" },
  );
  if (!res.ok) throw new Error(await formatApiError(res));
  return (await res.json()) as BacktestSettings;
}

async function formatApiError(res: Response): Promise<string> {
  const text = (await res.text()).trim();
  if (!text) return res.statusText || `HTTP ${res.status}`;
  try {
    const body = JSON.parse(text) as { error?: string };
    if (typeof body.error === "string" && body.error) return body.error;
  } catch {
    /* plain text */
  }
  return text;
}

const emptyLive: LiveState = {
  stage: "",
  symbol: "",
  candleClose: 0,
  since: null,
  lastStage: "",
  lastExit: null,
  lastFinished: null,
};

export function BacktestingPanel({
  className,
  playbookOptions,
}: {
  className?: string;
  playbookOptions: PlaybookOption[];
}) {
  const [list, setList] = useState<BacktestRecord[]>([]);
  const { kpiById, loadingIds } = useBacktestListKpis(list);
  const tableColumns = useMemo(
    () => buildBacktestTableColumns(kpiById, loadingIds),
    [kpiById, loadingIds],
  );
  const [selected, setSelected] = useState<BacktestRecord | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const [createName, setCreateName] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createSettings, setCreateSettings] = useState<BacktestSettings>(defaultBacktestSettings());
  const createRangeEditedRef = useRef(false);

  const [detailDraft, setDetailDraft] = useState(false);
  const [detailName, setDetailName] = useState("");
  const [detailSettings, setDetailSettings] = useState<BacktestSettings | null>(null);

  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");

  const [reportRows, setReportRows] = useState<ActivityRow[] | null>(null);
  const [reportJournals, setReportJournals] = useState<JournalRow[]>([]);
  const [reportTitles, setReportTitles] = useState<Record<string, string>>({});
  const [reportLoading, setReportLoading] = useState(false);

  const loadReport = useCallback(async (rec: BacktestRecord) => {
    setReportLoading(true);
    try {
      const res = await fetch(`/api/backtests/${rec.id}/activity`, { cache: "no-store" });
      if (!res.ok) {
        setReportRows([]);
        return;
      }
      const data = (await res.json()) as {
        rows: ActivityRow[];
        journals?: JournalRow[];
        playbookTitles?: Record<string, string>;
      };
      setReportRows(data.rows);
      setReportJournals(data.journals ?? []);
      setReportTitles(data.playbookTitles ?? {});
    } finally {
      setReportLoading(false);
    }
  }, []);

  const loadList = useCallback(async () => {
    const res = await fetch("/api/backtests", { cache: "no-store" });
    if (!res.ok) throw new Error(await res.text());
    const data = (await res.json()) as { backtests: BacktestRecord[] };
    const backtests = data.backtests ?? [];
    setList(backtests);
    setSelected((cur) => {
      if (!cur) return cur;
      const next = backtests.find((b) => b.id === cur.id) ?? cur;
      if (next.status === "completed" && cur.status !== "completed" && detailOpen) {
        void loadReport(next);
      }
      return next;
    });
  }, [detailOpen, loadReport]);

  useEffect(() => {
    void loadList().catch((e) => setError(e instanceof Error ? e.message : "load failed"));
    const ms = selected?.status === "running" ? 2000 : 5000;
    const t = window.setInterval(() => {
      void loadList().catch(() => {});
    }, ms);
    return () => window.clearInterval(t);
  }, [loadList, selected?.status]);

  const applyDraftSettings = useCallback((resolved: BacktestSettings, overwriteRange: boolean) => {
    if (!overwriteRange) return;
    setCreateSettings((s) => ({ ...s, fromMs: resolved.fromMs, toMs: resolved.toMs }));
  }, []);

  const openNew = () => {
    createRangeEditedRef.current = false;
    setCreateName(`Backtest ${new Date().toISOString().slice(0, 10)}`);
    setCreateDescription("");
    setCreateSettings(defaultBacktestSettings());
    setCreateOpen(true);
    void fetchDraftSettings("BTCUSDT")
      .then((resolved) => {
        if (createRangeEditedRef.current) return;
        applyDraftSettings(resolved, true);
      })
      .catch(() => {});
  };

  const onCreateSettingsChange = (patch: Partial<BacktestSettings>) => {
    if (patch.fromMs !== undefined || patch.toMs !== undefined) {
      createRangeEditedRef.current = true;
    }
    if (patch.symbol && patch.symbol !== createSettings.symbol) {
      const symbol = patch.symbol;
      createRangeEditedRef.current = false;
      setCreateSettings((s) => ({ ...s, ...patch }));
      void fetchDraftSettings(symbol)
        .then((resolved) => applyDraftSettings(resolved, true))
        .catch(() => {});
      return;
    }
    setCreateSettings((s) => ({ ...s, ...patch }));
  };

  const openRow = async (rec: BacktestRecord) => {
    setSelected(rec);
    setDetailDraft(false);
    setDetailName(rec.name);
    setDetailSettings(rec.settings);
    setDetailOpen(true);
    setReportRows(null);
    if (rec.status === "completed" || rec.status === "failed") {
      await loadReport(rec);
    }
  };

  const createBacktest = async () => {
    setSaving(true);
    setError("");
    try {
      const res = await fetch("/api/backtests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: createName,
          description: createDescription,
          settings: createSettings,
        }),
      });
      if (!res.ok) throw new Error(await formatApiError(res));
      const rec = (await res.json()) as BacktestRecord;
      setCreateOpen(false);
      await loadList();
      await openRow(rec);
    } catch (e) {
      setError(e instanceof Error ? e.message : "create failed");
    } finally {
      setSaving(false);
    }
  };

  const saveDetail = async () => {
    if (!selected || !detailSettings) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/backtests/${selected.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: detailName,
          description: selected.description,
          settings: detailSettings,
        }),
      });
      if (!res.ok) throw new Error(await formatApiError(res));
      const rec = (await res.json()) as BacktestRecord;
      setSelected(rec);
      setDetailDraft(false);
      await loadList();
    } catch (e) {
      setError(e instanceof Error ? e.message : "save failed");
    } finally {
      setSaving(false);
    }
  };

  const runBacktest = async () => {
    const id = selected?.id;
    if (!id) return;
    setRunning(true);
    setError("");
    try {
      const res = await fetch(`/api/backtests/${id}/run`, { method: "POST" });
      if (!res.ok) throw new Error(await formatApiError(res));
      const started = (await res.json()) as BacktestRecord;
      setSelected(started);
      setList((rows) => rows.map((b) => (b.id === started.id ? started : b)));
      setReportRows(null);
      void loadList();
    } catch (e) {
      setError(e instanceof Error ? e.message : "run failed");
    } finally {
      setRunning(false);
    }
  };

  const rec = selected;
  const outcome = reportRows ? summarizeBacktestOutcome(reportRows) : null;
  const kpiApiUrl =
    rec?.status === "completed"
      ? `/api/backtests/${rec.id}/kpis?fromMs=${rec.settings.fromMs}&toMs=${rec.settings.toMs}`
      : undefined;
  const kpiSet = useBacktestKpiSet(kpiApiUrl);
  const strategyTitle =
    rec &&
    (reportTitles[rec.settings.playbook] ??
      playbookOptions.find((p) => p.id === rec.settings.playbook)?.label ??
      rec.settings.playbook);
  const canEditDetail = rec && (rec.status === "draft" || rec.status === "failed");
  const showReport = rec?.status === "completed" && reportRows && reportRows.length > 0;
  const decisionTimestampsMs = useMemo(
    () => reportRows?.map((r) => r.candleClose).filter((ms) => ms > 0) ?? [],
    [reportRows],
  );

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col gap-4", className)}>
      <div className="flex shrink-0 items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">Backtests</h2>
          <p className="text-sm text-muted-foreground">
            Stored experiments with isolated workspaces; live Strategy data is never overwritten.
          </p>
        </div>
        <Button type="button" onClick={openNew}>New backtest</Button>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <FilterableTable
        rows={list}
        columns={tableColumns}
        rowKey={(r) => r.id}
        emptyPresentation="message"
        emptyMessage="No backtests yet. Create one to run ma-cross over historical data."
        onRowClick={(r) => void openRow(r)}
        className="min-h-0 flex-1"
      />

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>New backtest</DialogTitle>
            <DialogDescription>
              Configure symbol, range, capital, and execution costs. Save to add it to the catalog, then run from the
              detail panel.
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-2">
            <BacktestSettingsForm
              name={createName}
              description={createDescription}
              settings={createSettings}
              playbookOptions={playbookOptions}
              showDescription
              onNameChange={setCreateName}
              onDescriptionChange={setCreateDescription}
              onSettingsChange={onCreateSettingsChange}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button type="button" disabled={saving || !createName.trim()} onClick={() => void createBacktest()}>
              Create
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent
          side="right"
          className="flex w-full flex-col gap-0 p-0 data-[side=right]:!w-full sm:max-w-3xl lg:max-w-6xl"
        >
          {rec && (
            <>
              <SheetHeader className="border-b px-4 py-3 text-left">
                <SheetTitle className="pr-8 text-left">{rec.name}</SheetTitle>
              </SheetHeader>

              <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4 space-y-4">
                <BacktestStrategyMetadata
                  strategyTitle={strategyTitle ?? rec.settings.playbook}
                  playbook={rec.settings.playbook}
                  symbol={rec.settings.symbol}
                  fromMs={rec.settings.fromMs}
                  toMs={rec.settings.toMs}
                />

                {rec.status === "failed" && rec.error && (
                  <p className="text-destructive text-xs">{rec.error}</p>
                )}

                {(kpiApiUrl || outcome || reportLoading) && (
                  <BacktestPerformancePanel
                    kpi={kpiSet}
                    comparison={kpiSet.comparison}
                    statistics={kpiSet.statistics}
                    outcome={outcome}
                    decisionTimestampsMs={decisionTimestampsMs}
                    loading={reportLoading || kpiSet.loading}
                  />
                )}

                {canEditDetail && detailDraft && detailSettings && (
                  <BacktestSettingsForm
                    name={detailName}
                    settings={detailSettings}
                    playbookOptions={playbookOptions}
                    onNameChange={setDetailName}
                    onSettingsChange={(patch) => setDetailSettings((s) => (s ? { ...s, ...patch } : s))}
                  />
                )}

                {canEditDetail && !detailDraft && (
                  <Button type="button" variant="outline" size="sm" onClick={() => setDetailDraft(true)}>
                    Edit settings
                  </Button>
                )}

                {showReport && (
                  <ActivityView
                    rows={reportRows!}
                    journals={reportJournals}
                    playbookTitles={reportTitles}
                    live={emptyLive}
                    reportApi={{
                      backtestId: rec.id,
                      fromMs: rec.settings.fromMs,
                      toMs: rec.settings.toMs,
                    }}
                    className="min-h-0"
                  />
                )}

                {rec.status === "completed" && !reportLoading && reportRows?.length === 0 && (
                  <p className="text-sm text-muted-foreground">No activity rows for this run.</p>
                )}

                {rec.status === "draft" && !detailDraft && (
                  <p className="text-sm text-muted-foreground">
                    Draft — save settings if you changed them, then run the backtest job.
                  </p>
                )}
              </div>

              <SheetFooter className="border-t px-4 py-3 flex flex-col gap-2 sm:flex-col">
                {rec.status === "running" && (() => {
                  const pct = backtestProgressPercent(rec);
                  return pct != null ? (
                    <div className="w-full space-y-1">
                      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                        <div
                          className="h-full bg-primary transition-all duration-300"
                          style={{ width: `${pct}%` }}
                        />
                      </div>
                      {rec.progressStage === 4 && rec.progressTotal > 0 && (
                        <p className="text-xs text-muted-foreground">
                          Bar {rec.progressDone} / {rec.progressTotal}
                        </p>
                      )}
                    </div>
                  ) : null;
                })()}
                <div className="flex w-full flex-wrap items-center justify-between gap-2">
                  <div className="flex min-w-0 flex-wrap items-center gap-2 text-sm">
                    <span className="text-muted-foreground shrink-0">Status</span>
                    <BacktestStatusBadge
                      status={rec.status}
                      progressDone={rec.progressDone}
                      progressTotal={rec.progressTotal}
                      progressStage={rec.progressStage}
                      progressStages={rec.progressStages}
                      progressStageLabel={rec.progressStageLabel}
                    />
                    {rec.finishedAt != null && rec.status !== "running" && (
                      <span className="text-xs text-muted-foreground">
                        Finished {new Date(rec.finishedAt).toISOString().replace("T", " ").slice(0, 19)} UTC
                      </span>
                    )}
                  </div>
                  <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
                    {canEditDetail && detailDraft && (
                      <Button type="button" variant="secondary" disabled={saving} onClick={() => void saveDetail()}>
                        Save
                      </Button>
                    )}
                    {rec.status !== "running" && (
                      <Button
                        type="button"
                        disabled={running || detailDraft || (rec.status === "draft" && !rec.id)}
                        onClick={() => void runBacktest()}
                      >
                        {rec.status === "completed" || rec.status === "failed" ? "Re-run" : "Run"}
                      </Button>
                    )}
                  </div>
                </div>
              </SheetFooter>
            </>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
