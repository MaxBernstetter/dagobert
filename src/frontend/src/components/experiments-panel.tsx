
import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from "react";
import { apiUrl } from "@/lib/api";
import { chartMaxPoints, chartSeriesUrl, loadChartSeries } from "@/lib/chart-series-cache";
import { useViewportHeight } from "@/hooks/use-viewport-height";
import { ActivityView } from "@/components/activity-view";
import { BacktestDatePicker } from "@/components/backtest-date-picker";
import { BacktestPerformancePanel } from "@/components/backtest-performance-panel";
import { CatalogStatusBadge } from "@/components/backtest-status-badge";
import { ExperimentStatusBadge } from "@/components/experiment-status-badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FilterableTable, type SortDir } from "@/components/filterable-table";
import { MultiSelectDropdown } from "@/components/multi-select-dropdown";
import {
  formatOptionNumber,
  formatOptionRange,
  NumberOptionRangeSlider,
} from "@/components/number-option-range-slider";
import { NumberOptionSlider } from "@/components/number-option-slider";
import { ProviderSelect } from "@/components/provider-select";
import { SymbolPicker } from "@/components/symbol-picker";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  buildExperimentInstanceTableColumns,
  DEFAULT_VISIBLE_INSTANCE_COLUMNS,
  SERVER_SORT_KEY,
} from "@/components/experiment-instance-table-columns";
import { MonthlyReturnChart } from "@/components/monthly-return-chart";
import { ExperimentPerformanceMetrics } from "@/components/experiment-performance-metrics";
import { useBacktestKpiSet } from "@/hooks/use-backtest-kpi-set";
import {
  backtestDateInputToFromMs,
  backtestDateInputToToMs,
  backtestFromMsToDateInput,
  backtestToMsToDateInput,
  formatBacktestRange,
} from "@/lib/backtest-shared";
import type { ActivityRow, JournalRow, LiveState } from "@/types/read";
import {
  GRID_SYMBOL_KEY,
  axisFromDraft,
  splitGridCombo,
  type GridAxisDraft,
  previewGrid,
} from "@/lib/experiment-grid";
import { clampGridRange, clampNumber } from "@/lib/experiment-option-validation";
import { splitPipelineSymbol, type MarketPair } from "@/lib/market-pair";
import {
  DEFAULT_MARKET_PROVIDER,
  marketProviderLabel,
} from "@/lib/market-providers";
import { fetchReadyProviders } from "@/lib/ready-providers";
import { loadProviderPairs, prefetchProviderPairs } from "@/lib/provider-pairs";
import { experimentHasResults } from "@/lib/catalog-status";
import {
  defaultExperimentSettings,
  normalizeExperiment,
  normalizeProgress,
  normalizeInstance,
  experimentCreateModeProto,
  toProtoGridAxes,
  experimentPipelineSymbol,
  experimentProgressPercent,
  type MaCrossConfig,
  type SignalConsensusConfig,
  type ExperimentInstance,
  type ExperimentInstancePage,
  type ExperimentProgress,
  type ExperimentRecord,
  type ExperimentSettings,
} from "@/lib/experiment-shared";
import {
  defaultSymbolPairs,
  pairsToPipelineSymbols,
  pipelineSymbolsToPairs,
} from "@/lib/symbol-picker";
import { cn } from "@/lib/utils";
import { usePageBreadcrumbs, type PageCrumb } from "@/components/page-breadcrumbs";

const fieldInputClass =
  "w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50";

const INSTANCE_PAGE_SIZE = 50;

const emptyLive: LiveState = {
  stage: "",
  symbol: "",
  candleClose: 0,
  since: null,
  lastStage: "",
  lastExit: null,
  lastFinished: null,
};

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

type StrategyOptionField =
  | {
      key: string;
      label: string;
      description?: string;
      kind: "enum";
      valueType: "number" | "string";
      values: Array<{ value: number | string; label: string }>;
      default: number | string;
    }
  | {
      key: string;
      label: string;
      description?: string;
      kind: "number";
      min: number;
      max: number;
      step?: number;
      unit?: string;
      default: number;
    };

type ExperimentStrategy = {
  id: string;
  label: string;
  description?: string;
  options: StrategyOptionField[];
};

type InstanceDraft = Record<string, number | string>;

function defaultInstance(options: StrategyOptionField[]): InstanceDraft {
  const out: InstanceDraft = {};
  for (const opt of options) {
    out[opt.key] = opt.default;
  }
  return out;
}

function instanceLabel(d: InstanceDraft, options: StrategyOptionField[]): string {
  const parts: string[] = [];
  const sym = d[GRID_SYMBOL_KEY];
  if (sym != null && String(sym)) parts.push(String(sym));
  for (const opt of options) {
    const raw = d[opt.key] ?? opt.default;
    if (opt.kind === "enum") {
      const match = opt.values.find((v) => v.value === raw);
      parts.push(match?.label ?? String(raw));
    } else {
      parts.push(opt.unit ? `${raw}${opt.unit}` : String(raw));
    }
  }
  return parts.length > 0 ? parts.join(" / ") : "config";
}

function defaultGridDrafts(
  options: StrategyOptionField[],
  catalog: MarketPair[] = [],
): GridAxisDraft[] {
  const symbolAxis: GridAxisDraft = {
    key: GRID_SYMBOL_KEY,
    kind: "enum",
    selected: pairsToPipelineSymbols(defaultSymbolPairs(catalog)),
  };
  const strategyAxes: GridAxisDraft[] = options.map((opt) => {
    if (opt.kind === "enum") {
      return {
        key: opt.key,
        kind: "enum" as const,
        selected: opt.values.map((v) => v.value),
      };
    }
    // Start at the option default (single point); user widens from/to for GRID.
    const tick = opt.step && opt.step > 0 ? opt.step : 1;
    return {
      key: opt.key,
      kind: "number" as const,
      from: opt.default,
      to: opt.default,
      tick,
    };
  });
  return [symbolAxis, ...strategyAxes];
}

function setEnumAxisSelected(
  drafts: GridAxisDraft[],
  key: string,
  selected: Array<number | string>,
): GridAxisDraft[] {
  return drafts.map((d) =>
    d.key === key && d.kind === "enum" ? { ...d, selected } : d,
  );
}

type CreateMode = "manual" | "grid";

export function ExperimentsPanel({ className }: { className?: string }) {
  const { setCrumbs } = usePageBreadcrumbs();
  const [list, setList] = useState<ExperimentRecord[]>([]);
  const [selected, setSelected] = useState<ExperimentRecord | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createSettings, setCreateSettings] = useState<ExperimentSettings>(defaultExperimentSettings());
  // Stable handlers: the date pickers are memoized and must not re-render on the polling tick.
  const onCreateFromDateChange = useCallback((dateInput: string) => {
    const fromMs = backtestDateInputToFromMs(dateInput);
    if (fromMs == null) return;
    setCreateSettings((s) => {
      const next = { ...s, fromMs };
      if (fromMs >= s.toMs) {
        const bumped = backtestDateInputToToMs(dateInput);
        if (bumped != null && bumped > fromMs) next.toMs = bumped;
      }
      return next;
    });
  }, []);
  const onCreateToDateChange = useCallback((dateInput: string) => {
    const toMs = backtestDateInputToToMs(dateInput);
    setCreateSettings((s) => (toMs == null || toMs <= s.fromMs ? s : { ...s, toMs }));
  }, []);
  const [createPlaybook, setCreatePlaybook] = useState("ma-cross");
  const [strategies, setStrategies] = useState<ExperimentStrategy[]>([]);
  const [instanceDrafts, setInstanceDrafts] = useState<InstanceDraft[]>([]);
  const [createMode, setCreateMode] = useState<CreateMode>("manual");
  const [gridDrafts, setGridDrafts] = useState<GridAxisDraft[]>([]);
  const [catalogPairs, setCatalogPairs] = useState<MarketPair[]>([]);
  const [gridSymbolsLoading, setGridSymbolsLoading] = useState(false);
  const [activeInstanceId, setActiveInstanceId] = useState<string | null>(null);
  const [instanceDetailOpen, setInstanceDetailOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [error, setError] = useState("");
  const [reportRows, setReportRows] = useState<ActivityRow[] | null>(null);
  const [reportJournals, setReportJournals] = useState<JournalRow[]>([]);
  const [reportTitles, setReportTitles] = useState<Record<string, string>>({});
  const [reportLoading, setReportLoading] = useState(false);
  const [instanceRows, setInstanceRows] = useState<ExperimentInstance[]>([]);
  const [instanceTotal, setInstanceTotal] = useState(0);
  const [instanceOffset, setInstanceOffset] = useState(0);
  const [instanceSort, setInstanceSort] = useState<string | null>("label");
  const [instanceOrder, setInstanceOrder] = useState<SortDir>("asc");
  const [visibleInstanceColumns, setVisibleInstanceColumns] = useState<string[]>(
    DEFAULT_VISIBLE_INSTANCE_COLUMNS,
  );
  const [instanceQ, setInstanceQ] = useState("");
  const [instanceQDebounced, setInstanceQDebounced] = useState("");
  const [failedInstanceCount, setFailedInstanceCount] = useState(0);
  const [activeInstanceDetail, setActiveInstanceDetail] = useState<ExperimentInstance | null>(
    null,
  );

  const selectedStrategy = useMemo(
    () => strategies.find((s) => s.id === createPlaybook) ?? strategies[0] ?? null,
    [strategies, createPlaybook],
  );
  const strategyOptions = selectedStrategy?.options ?? [];
  const gridPreview = useMemo(
    () => previewGrid(gridDrafts, createPlaybook),
    [gridDrafts, createPlaybook],
  );
  const selectedSymbols = useMemo(() => {
    const draft = gridDrafts.find((d) => d.key === GRID_SYMBOL_KEY);
    if (!draft || draft.kind !== "enum") return [] as string[];
    return draft.selected.map(String).filter((s) => s.trim() !== "");
  }, [gridDrafts]);
  const selectedSymbolPairs = useMemo(
    () => pipelineSymbolsToPairs(selectedSymbols, catalogPairs),
    [selectedSymbols, catalogPairs],
  );
  const manualInstanceCount = selectedSymbols.length * instanceDrafts.length;

  useEffect(() => {
    const first = selectedSymbols[0];
    if (!first) return;
    const pair = splitPipelineSymbol(first);
    if (!pair) return;
    setCreateSettings((s) =>
      s.base === pair.base && s.quote === pair.quote
        ? s
        : { ...s, base: pair.base, quote: pair.quote },
    );
  }, [selectedSymbols]);

  function setSelectedSymbolPairs(next: MarketPair[]) {
    setGridDrafts((prev) =>
      setEnumAxisSelected(prev, GRID_SYMBOL_KEY, pairsToPipelineSymbols(next)),
    );
  }

  const refresh = useCallback(async () => {
    const res = await fetch(apiUrl("/experiments"), { cache: "no-store" });
    if (!res.ok) throw new Error(await formatApiError(res));
    const j = await res.json();
    const data = { experiments: ((j.experiments ?? []) as Record<string, unknown>[]).map((s) => normalizeExperiment(s)) };
    setList(data.experiments ?? []);
    return data.experiments ?? [];
  }, []);

  useEffect(() => {
    void refresh().catch((err) => setError(err instanceof Error ? err.message : "load failed"));
  }, [refresh]);

  // Warm pairs for ready providers (disk snapshot after Ingestion activate).
  useEffect(() => {
    void (async () => {
      try {
        const ids = (await fetchReadyProviders()).map((p) => p.id);
        for (const id of ids.length > 0 ? ids : [DEFAULT_MARKET_PROVIDER]) {
          prefetchProviderPairs(id);
        }
      } catch {
        prefetchProviderPairs(DEFAULT_MARKET_PROVIDER);
      }
    })();
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch(apiUrl("/experiments/strategies"), { cache: "no-store" });
        if (!res.ok) return;
        const data = (await res.json()) as { strategies: ExperimentStrategy[] };
        const ready = data.strategies ?? [];
        setStrategies(ready);
        const first = ready[0];
        if (first) {
          setCreatePlaybook((prev) => (ready.some((s) => s.id === prev) ? prev : first.id));
          setInstanceDrafts([
            defaultInstance(first.options),
            defaultInstance(first.options),
          ]);
          setGridDrafts(defaultGridDrafts(first.options));
        }
      } catch {
        /* ignore */
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load once on mount
  }, []);

  useEffect(() => {
    if (!createOpen) return;
    const provider = createSettings.provider;
    let cancelled = false;
    setGridSymbolsLoading(true);
    void loadProviderPairs(provider)
      .then((pairs) => {
        if (cancelled) return;
        const catalog = pairs.map((p) => ({
          base: p.base.toUpperCase(),
          quote: p.quote.toUpperCase(),
        }));
        setCatalogPairs(catalog);
        setGridDrafts((prev) => {
          const symbolDraft = prev.find((d) => d.key === GRID_SYMBOL_KEY);
          if (!symbolDraft || symbolDraft.kind !== "enum") {
            return defaultGridDrafts(strategyOptions, catalog);
          }
          const stillValid = pipelineSymbolsToPairs(
            symbolDraft.selected.map(String),
            catalog,
          );
          const selected =
            stillValid.length > 0
              ? pairsToPipelineSymbols(stillValid)
              : pairsToPipelineSymbols(defaultSymbolPairs(catalog));
          return setEnumAxisSelected(prev, GRID_SYMBOL_KEY, selected);
        });
      })
      .catch(() => {
        if (cancelled) return;
        setCatalogPairs([]);
      })
      .finally(() => {
        if (!cancelled) setGridSymbolsLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // strategyOptions identity changes often; only re-load on dialog/provider.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional
  }, [createOpen, createSettings.provider]);

  useEffect(() => {
    const t = setTimeout(() => setInstanceQDebounced(instanceQ), 250);
    return () => clearTimeout(t);
  }, [instanceQ]);

  const loadInstancePage = useCallback(
    async (experimentId: string) => {
      const params = new URLSearchParams({
        limit: String(INSTANCE_PAGE_SIZE),
        cursor: String(instanceOffset),
      });
      const serverSort = instanceSort ? SERVER_SORT_KEY[instanceSort] : undefined;
      if (serverSort) {
        params.set("sort", serverSort);
        params.set("order", instanceOrder);
      }
      if (instanceQDebounced.trim()) params.set("q", instanceQDebounced.trim());
      const res = await fetch(apiUrl(`/experiments/${experimentId}/instances?${params}`), {
        cache: "no-store",
      });
      if (!res.ok) return;
      const pageRaw = await res.json() as ExperimentInstancePage & { instances?: Record<string, unknown>[] };
      const page: ExperimentInstancePage = {
        ...pageRaw,
        instances: (pageRaw.instances ?? []).map((i) =>
          normalizeInstance(i as unknown as Record<string, unknown>),
        ),
        nextCursor: pageRaw.nextCursor ?? null,
      };
      setInstanceRows(page.instances ?? []);
      setInstanceTotal(page.total ?? 0);
    },
    [instanceOffset, instanceSort, instanceOrder, instanceQDebounced],
  );

  useEffect(() => {
    if (!selected || !detailOpen) return;
    if (selected.status === "materializing") return;
    void loadInstancePage(selected.id).catch(() => undefined);
  }, [selected?.id, selected?.status, detailOpen, loadInstancePage]);

  useEffect(() => {
    if (
      !selected ||
      (selected.status !== "running" && selected.status !== "materializing")
    ) {
      return;
    }
    const experimentId = selected.id;
    let inFlight = false;
    const tick = async () => {
      if (inFlight) return;
      inFlight = true;
      try {
        const res = await fetch(apiUrl(`/experiments/${experimentId}/progress`), {
          cache: "no-store",
        });
        if (!res.ok) return;
        const progress = normalizeProgress(await res.json());
        setFailedInstanceCount(progress.failedInstanceCount ?? 0);
        setSelected((prev) => {
          if (!prev || prev.id !== progress.id) return prev;
          if (
            prev.status === progress.status &&
            prev.progressInstanceDone === progress.progressInstanceDone &&
            prev.progressStage === progress.progressStage &&
            prev.progressStageLabel === progress.progressStageLabel &&
            prev.error === progress.error
          ) {
            return prev;
          }
          return {
            ...prev,
            status: progress.status,
            error: progress.error,
            progressStage: progress.progressStage,
            progressStages: progress.progressStages,
            progressStageLabel: progress.progressStageLabel,
            progressInstanceDone: progress.progressInstanceDone,
            progressInstanceTotal: progress.progressInstanceTotal,
            startedAtMs: progress.startedAtMs,
            finishedAtMs: progress.finishedAtMs,
          };
        });
        setList((prev) =>
          prev.map((s) =>
            s.id === progress.id
              ? {
                  ...s,
                  status: progress.status,
                  error: progress.error,
                  progressStage: progress.progressStage,
                  progressStages: progress.progressStages,
                  progressStageLabel: progress.progressStageLabel,
                  progressInstanceDone: progress.progressInstanceDone,
                  progressInstanceTotal: progress.progressInstanceTotal,
                  startedAtMs: progress.startedAtMs,
                  finishedAtMs: progress.finishedAtMs,
                }
              : s,
          ),
        );
        if (progress.status !== "materializing") {
          void loadInstancePage(experimentId).catch(() => undefined);
        }
      } catch {
        /* ignore */
      } finally {
        inFlight = false;
      }
    };
    void tick();
    const t = setInterval(() => {
      void tick();
    }, 1500);
    return () => clearInterval(t);
  }, [selected?.id, selected?.status, loadInstancePage]);

  const activeInstance: ExperimentInstance | null = useMemo(() => {
    if (!activeInstanceId) return null;
    if (activeInstanceDetail?.id === activeInstanceId) return activeInstanceDetail;
    return instanceRows.find((i) => i.id === activeInstanceId) ?? null;
  }, [activeInstanceId, activeInstanceDetail, instanceRows]);

  const instanceColumns = useMemo(
    () =>
      buildExperimentInstanceTableColumns(
        selected ? experimentPipelineSymbol(selected.settings) : "",
      ),
    [selected?.settings.base, selected?.settings.quote],
  );

  const shownInstanceColumns = useMemo(
    () => instanceColumns.filter((c) => visibleInstanceColumns.includes(c.id)),
    [instanceColumns, visibleInstanceColumns],
  );

  function toggleInstanceColumn(id: string) {
    setVisibleInstanceColumns((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    );
  }

  // Heights follow the window, so the charts use the space the wider detail layout gives them.
  const monthlyChartHeight = useViewportHeight(180, 360, 0.25);
  // A finished instance has its report already. The experiment may still be running or cancelled.
  const instanceRunReady = activeInstance?.status === "completed";
  const showInstancePerformance =
    instanceDetailOpen && selected != null && instanceRunReady;

  const kpiUrl =
    instanceDetailOpen && selected && instanceRunReady
      ? apiUrl(`/experiments/${selected.id}/instances/${activeInstance.id}/kpis`)
      : undefined;
  const kpiState = useBacktestKpiSet(kpiUrl);
  const experimentPerfUrl =
    selected && experimentHasResults(selected.status)
      ? apiUrl(`/experiments/${selected.id}/performance`)
      : undefined;
  const experimentPerf = useBacktestKpiSet(experimentPerfUrl);

  useEffect(() => {
    if (!instanceDetailOpen || !selected || !activeInstance || activeInstance.status !== "completed") {
      setReportRows(null);
      setReportJournals([]);
      setReportTitles({});
      return;
    }
    let cancelled = false;
    setReportLoading(true);
    void (async () => {
      try {
        const res = await fetch(
          apiUrl(`/experiments/${selected.id}/instances/${activeInstance.id}/activity`),
          { cache: "no-store" },
        );
        if (!res.ok) throw new Error(await formatApiError(res));
        const data = (await res.json()) as {
          rows: ActivityRow[];
          journals: JournalRow[];
          playbookTitles: Record<string, string>;
        };
        if (!cancelled) {
          setReportRows(data.rows ?? []);
          setReportJournals(data.journals ?? []);
          setReportTitles(data.playbookTitles ?? {});
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "activity load failed");
          setReportRows([]);
        }
      } finally {
        if (!cancelled) setReportLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [instanceDetailOpen, selected, activeInstance]);

  useEffect(() => {
    if (!instanceDetailOpen || !selected || !activeInstance || activeInstance.status !== "completed") return;
    // Same URL as the instance charts (window and budget), so the prefetch fills their cache entry.
    const url = chartSeriesUrl(
      apiUrl(`/experiments/${selected.id}/instances/${activeInstance.id}/chart-series`),
      selected.settings.fromMs,
      selected.settings.toMs,
      chartMaxPoints(),
    );
    void loadChartSeries(url).catch(() => {});
  }, [instanceDetailOpen, selected, activeInstance]);

  async function openExperiment(row: ExperimentRecord) {
    setSelected(row);
    setActiveInstanceId(null);
    setActiveInstanceDetail(null);
    setInstanceDetailOpen(false);
    setInstanceRows([]);
    setInstanceTotal(row.progressInstanceTotal || 0);
    setInstanceOffset(0);
    setInstanceQ("");
    setInstanceQDebounced("");
    setFailedInstanceCount(0);
    setDetailOpen(true);
    try {
      const [experimentRes, progressRes] = await Promise.all([
        fetch(apiUrl(`/experiments/${row.id}`), { cache: "no-store" }),
        fetch(apiUrl(`/experiments/${row.id}/progress`), { cache: "no-store" }),
      ]);
      if (experimentRes.ok) {
        const rec = normalizeExperiment(await experimentRes.json());
        setSelected(rec);
        setInstanceTotal(rec.progressInstanceTotal || 0);
      }
      if (progressRes.ok) {
        const progress = normalizeProgress(await progressRes.json());
        setFailedInstanceCount(progress.failedInstanceCount ?? 0);
      }
    } catch {
      /* keep list snapshot */
    }
  }

  async function openInstanceDetail(inst: ExperimentInstance) {
    setActiveInstanceId(inst.id);
    setActiveInstanceDetail(inst);
    setInstanceDetailOpen(true);
    setReportRows(null);
    setReportJournals([]);
    setReportTitles({});
    setReportLoading(inst.status === "completed");
    if (!selected || inst.status !== "completed") return;
    try {
      const res = await fetch(
        apiUrl(`/experiments/${selected.id}/instances/${inst.id}/report`),
        { cache: "no-store" },
      );
      if (!res.ok) return;
      const report = (await res.json()) as NonNullable<ExperimentInstance["instanceReport"]>;
      setActiveInstanceDetail((prev) =>
        prev && prev.id === inst.id
          ? {
              ...prev,
              instanceReport: report,
              runReport: report.run,
              durationMs: report.run.durationMs ?? prev.durationMs,
            }
          : prev,
      );
    } catch {
      /* ignore */
    }
  }

  function closeInstanceDetail() {
    setInstanceDetailOpen(false);
    setActiveInstanceId(null);
    setActiveInstanceDetail(null);
    setReportRows(null);
    setReportJournals([]);
    setReportTitles({});
  }

  function closeExperiment() {
    closeInstanceDetail();
    setDetailOpen(false);
  }

  useLayoutEffect(() => {
    const crumbs: PageCrumb[] = [
      {
        label: "Experiments",
        onClick: detailOpen ? closeExperiment : undefined,
      },
    ];
    if (detailOpen && selected) {
      crumbs.push({
        label: selected.name || "Experiment",
        onClick: instanceDetailOpen ? closeInstanceDetail : undefined,
      });
    }
    if (detailOpen && instanceDetailOpen && activeInstance) {
      crumbs.push({ label: activeInstance.label });
    }
    setCrumbs(crumbs);
  }, [detailOpen, selected, instanceDetailOpen, activeInstance, setCrumbs]);

  useEffect(() => {
    if (createOpen || deleteConfirmOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      if (instanceDetailOpen) {
        event.preventDefault();
        closeInstanceDetail();
        return;
      }
      if (detailOpen) {
        event.preventDefault();
        setDetailOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [createOpen, deleteConfirmOpen, detailOpen, instanceDetailOpen]);

  function draftToInstance(d: InstanceDraft, fallbackSymbol: string) {
    const { symbol: fromDraft, strategyConfig: rawCfg } = splitGridCombo(d);
    const strategyConfig: Record<string, number | string> = {};
    for (const opt of strategyOptions) {
      const raw = rawCfg[opt.key] ?? d[opt.key] ?? opt.default;
      strategyConfig[opt.key] =
        opt.kind === "enum" && opt.valueType === "number"
          ? Number(raw)
          : opt.kind === "number"
            ? Number(raw)
            : raw;
    }
    const symbol = (fromDraft ?? fallbackSymbol).trim() || fallbackSymbol;
    const playbook = selectedStrategy!.id;
    const label = instanceLabel({ ...d, [GRID_SYMBOL_KEY]: symbol }, strategyOptions);
    const slug = playbook.replace(/\.ya?ml$/i, "");
    if (slug === "ma-cross") {
      return { label, playbook, symbol, maCross: strategyConfig as unknown as MaCrossConfig };
    }
    return {
      label,
      playbook,
      symbol,
      signalConsensus: strategyConfig as unknown as SignalConsensusConfig,
    };
  }

  async function handleCreate() {
    setSaving(true);
    setError("");
    try {
      if (!selectedStrategy) {
        throw new Error("No experiment-ready strategy available");
      }
      const settings = { ...createSettings };
      let body: Record<string, unknown>;
      if (createMode === "grid") {
        if (!gridPreview.ok) {
          throw new Error(gridPreview.error ?? "Invalid grid");
        }
        body = {
          name: createName.trim() || `Experiment ${new Date().toISOString().slice(0, 10)}`,
          description: createDescription,
          settings,
          mode: experimentCreateModeProto("grid"),
          playbook: selectedStrategy.id,
          grid: {
            playbook: selectedStrategy.id,
            axes: toProtoGridAxes(gridDrafts.map(axisFromDraft)),
          },
        };
      } else {
        if (selectedSymbols.length === 0) {
          throw new Error("Select at least one symbol");
        }
        if (instanceDrafts.length === 0) {
          throw new Error("Add at least one instance");
        }
        const instances: ReturnType<typeof draftToInstance>[] = [];
        for (const sym of selectedSymbols) {
          for (const d of instanceDrafts) {
            instances.push(draftToInstance({ ...d, [GRID_SYMBOL_KEY]: sym }, sym));
          }
        }
        body = {
          name: createName.trim() || `Experiment ${new Date().toISOString().slice(0, 10)}`,
          description: createDescription,
          settings,
          mode: experimentCreateModeProto("manual"),
          playbook: selectedStrategy.id,
          instances,
        };
      }
      const res = await fetch(apiUrl("/experiments"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(await formatApiError(res));
      const rec = normalizeExperiment(await res.json());
      setList((prev) => [rec, ...prev.filter((s) => s.id !== rec.id)]);
      setCreateOpen(false);
      void openExperiment(rec);
    } catch (err) {
      setError(err instanceof Error ? err.message : "create failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleRun(opts?: { retryFailed?: boolean }) {
    if (!selected) return;
    setRunning(true);
    setError("");
    try {
      const res = await fetch(apiUrl(`/experiments/${selected.id}/run`), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ retryFailed: opts?.retryFailed === true }),
      });
      if (!res.ok) throw new Error(await formatApiError(res));
      const experiments = await refresh();
      const updated = experiments.find((s) => s.id === selected.id) ?? null;
      setSelected(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "run failed");
    } finally {
      setRunning(false);
    }
  }

  async function handleCancel() {
    if (!selected || selected.status !== "running") return;
    setCancelling(true);
    setError("");
    try {
      const res = await fetch(apiUrl(`/experiments/${selected.id}/cancel`), { method: "POST" });
      if (!res.ok) throw new Error(await formatApiError(res));
      const experiments = await refresh();
      const updated = experiments.find((s) => s.id === selected.id) ?? null;
      setSelected(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "cancel failed");
    } finally {
      setCancelling(false);
    }
  }

  function requestDelete() {
    if (!selected) return;
    if (selected.status === "running") {
      setDeleteConfirmOpen(true);
      return;
    }
    void handleDelete(false);
  }

  async function handleDelete(cancelThenDelete: boolean) {
    if (!selected) return;
    setDeleting(true);
    setError("");
    setDeleteConfirmOpen(false);
    const id = selected.id;
    try {
      const q = cancelThenDelete ? "?cancel=1" : "";
      const res = await fetch(`${apiUrl(`/experiments/${id}`)}${q}`, { method: "DELETE" });
      if (!res.ok) throw new Error(await formatApiError(res));
      await refresh();
      setSelected(null);
      setDetailOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "delete failed");
      const experiments = await refresh().catch(() => [] as ExperimentRecord[]);
      setSelected(experiments.find((s) => s.id === id) ?? null);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className={cn("flex min-h-0 flex-col gap-4", className)}>
      {!(detailOpen && selected) && (
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Experiments</h1>
          <p className="text-sm text-muted-foreground">
            Shared lake and KPI; multiple strategy configs in one experiment.
          </p>
        </div>
        <Button
          type="button"
          onClick={() => {
            setCreateName("");
            setCreateDescription("");
            setCreateSettings(defaultExperimentSettings());
            setCreateMode("manual");
            const first = strategies[0];
            const playbook = first?.id ?? "ma-cross";
            setCreatePlaybook(playbook);
            if (first) {
              setInstanceDrafts([
                defaultInstance(first.options),
                defaultInstance(first.options),
              ]);
            } else {
              setInstanceDrafts([]);
            }
            setGridDrafts(
              first ? defaultGridDrafts(first.options, catalogPairs) : [],
            );
            setCreateOpen(true);
          }}
        >
          New experiment
        </Button>
      </div>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      {!(detailOpen && selected) && (
      <div className="min-h-0 flex-1 overflow-auto rounded-md border">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-muted/80 text-left">
            <tr>
              <th className="px-3 py-2 font-medium">Name</th>
              <th className="px-3 py-2 font-medium">Strategy</th>
              <th className="px-3 py-2 font-medium">Provider</th>
              <th className="px-3 py-2 font-medium">Market</th>
              <th className="px-3 py-2 font-medium">Instances</th>
              <th className="px-3 py-2 font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {list.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-8 text-center text-muted-foreground">
                  No experiments yet.
                </td>
              </tr>
            )}
            {list.map((row) => {
              const symbols =
                row.listSymbols && row.listSymbols.length > 0
                  ? row.listSymbols
                  : [experimentPipelineSymbol(row.settings)];
              const playbooks =
                row.listPlaybooks && row.listPlaybooks.length > 0
                  ? row.listPlaybooks
                  : [];
              const instanceCount = row.progressInstanceTotal;
              return (
                <tr
                  key={row.id}
                  className="cursor-pointer border-t hover:bg-muted/40"
                  onClick={() => openExperiment(row)}
                >
                  <td className="px-3 py-2">{row.name}</td>
                  <td className="px-3 py-2 font-mono text-xs">
                    {playbooks.join(", ") || "—"}
                  </td>
                  <td className="px-3 py-2 text-xs">
                    {marketProviderLabel(row.settings.provider)}
                  </td>
                  <td className="px-3 py-2">
                    <span className="inline-flex flex-wrap gap-1">
                      {symbols.map((sym) => (
                        <Badge
                          key={sym}
                          variant="secondary"
                          className="font-mono text-xs font-normal"
                        >
                          {sym}
                        </Badge>
                      ))}
                    </span>
                  </td>
                  <td className="px-3 py-2 tabular-nums">{instanceCount}</td>
                  <td className="px-3 py-2">
                    <ExperimentStatusBadge record={row} />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-h-[90vh] gap-0 overflow-y-auto p-6 sm:max-w-3xl">
          <DialogHeader className="border-0 p-0 pb-4">
            <DialogTitle>New experiment</DialogTitle>
            <DialogDescription>
              Choose Manual or GRID, then set market, range, and strategy options.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-5 text-sm">
            <div className="grid gap-4">
              <label className="grid gap-1.5">
                <span className="text-muted-foreground">Name</span>
                <input
                  className={fieldInputClass}
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="Experiment YYYY-MM-DD"
                />
              </label>
              <label className="grid gap-1.5">
                <span className="text-muted-foreground">Description</span>
                <textarea
                  className={fieldInputClass}
                  rows={2}
                  value={createDescription}
                  onChange={(e) => setCreateDescription(e.target.value)}
                />
              </label>
            </div>

            <div className="border-t border-border" role="separator" />

            <div className="grid gap-4">
              <div
                className="grid w-full grid-cols-2 overflow-hidden rounded-md border border-border"
                role="group"
                aria-label="Instance configuration mode"
              >
                <button
                  type="button"
                  className={cn(
                    "px-3 py-2 text-sm font-medium transition-colors",
                    createMode === "manual"
                      ? "bg-primary text-primary-foreground"
                      : "bg-background text-muted-foreground hover:bg-muted",
                  )}
                  onClick={() => setCreateMode("manual")}
                >
                  Manual
                </button>
                <button
                  type="button"
                  className={cn(
                    "border-l border-border px-3 py-2 text-sm font-medium transition-colors",
                    createMode === "grid"
                      ? "bg-primary text-primary-foreground"
                      : "bg-background text-muted-foreground hover:bg-muted",
                  )}
                  onClick={() => setCreateMode("grid")}
                >
                  GRID
                </button>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:items-start">
                <ProviderSelect
                  value={createSettings.provider}
                  onChange={(provider) => setCreateSettings((s) => ({ ...s, provider }))}
                />
                <SymbolPicker
                  pairs={catalogPairs}
                  value={selectedSymbolPairs}
                  onChange={setSelectedSymbolPairs}
                  disabled={gridSymbolsLoading || catalogPairs.length === 0}
                  loading={gridSymbolsLoading}
                  emptyHint={
                    gridSymbolsLoading
                      ? "Loading markets…"
                      : "No markets — activate this provider under Connectors"
                  }
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <BacktestDatePicker
                  id="experiment-from"
                  label="From (UTC)"
                  value={backtestFromMsToDateInput(createSettings.fromMs)}
                  max={backtestToMsToDateInput(createSettings.toMs) || undefined}
                  onChange={onCreateFromDateChange}
                />
                <BacktestDatePicker
                  id="experiment-to"
                  label="To (UTC, inclusive)"
                  value={backtestToMsToDateInput(createSettings.toMs)}
                  min={backtestFromMsToDateInput(createSettings.fromMs) || undefined}
                  onChange={onCreateToDateChange}
                />
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={createSettings.redownloadMissing}
                  onChange={(e) =>
                    setCreateSettings((s) => ({
                      ...s,
                      redownloadMissing: e.target.checked,
                    }))
                  }
                />
                <span className="text-muted-foreground">
                  Force re-download candles in range (otherwise only missing hours are
                  fetched)
                </span>
              </label>
            </div>

            <div className="border-t border-border" role="separator" />

            <div className="grid gap-4">
              <label className="grid gap-1">
                <span className="text-muted-foreground">Strategy</span>
                <select
                  className={fieldInputClass}
                  value={createPlaybook}
                  disabled={strategies.length === 0}
                  onChange={(e) => {
                    const id = e.target.value;
                    setCreatePlaybook(id);
                    const strat = strategies.find((s) => s.id === id);
                    if (strat) {
                      setInstanceDrafts([
                        defaultInstance(strat.options),
                        defaultInstance(strat.options),
                      ]);
                      setGridDrafts(defaultGridDrafts(strat.options, catalogPairs));
                    }
                  }}
                >
                  {strategies.length === 0 && (
                    <option value="">No experiment-ready strategies</option>
                  )}
                  {strategies.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.label}
                    </option>
                  ))}
                </select>
              </label>

            {createMode === "manual" && (
              <>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Instances</span>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      setInstanceDrafts((prev) => [...prev, defaultInstance(strategyOptions)])
                    }
                  >
                    Add
                  </Button>
                </div>
                {instanceDrafts.map((draft, idx) => (
                  <div key={idx} className="grid gap-2">
                    <div className="grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2">
                      {strategyOptions.map((opt) =>
                        opt.kind === "enum" ? (
                          <Card
                            key={opt.key}
                            size="sm"
                            className="h-full min-w-0 gap-3 py-3.5"
                          >
                            <CardHeader className="gap-1.5 px-3.5 pb-0">
                              <CardTitle
                                className="truncate text-sm leading-5"
                                title={opt.description}
                              >
                                {opt.label}
                              </CardTitle>
                              <CardDescription className="h-8 overflow-hidden text-xs leading-4">
                                <span className="line-clamp-2">
                                  {opt.description || "\u00a0"}
                                </span>
                              </CardDescription>
                            </CardHeader>
                            <CardContent className="mt-auto px-3.5">
                              <select
                                className={fieldInputClass}
                                value={String(draft[opt.key] ?? opt.default)}
                                onChange={(e) => {
                                  const raw = e.target.value;
                                  const parsed =
                                    opt.valueType === "number" ? Number(raw) : raw;
                                  setInstanceDrafts((prev) =>
                                    prev.map((d, i) =>
                                      i === idx ? { ...d, [opt.key]: parsed } : d,
                                    ),
                                  );
                                }}
                              >
                                {opt.values.map((v) => (
                                  <option key={String(v.value)} value={String(v.value)}>
                                    {v.label}
                                  </option>
                                ))}
                              </select>
                            </CardContent>
                          </Card>
                        ) : (
                          <Card
                            key={opt.key}
                            size="sm"
                            className="h-full min-w-0 gap-3 py-3.5"
                          >
                            <CardHeader className="gap-1.5 px-3.5 pb-0">
                              <div className="flex h-5 items-baseline justify-between gap-2">
                                <CardTitle
                                  className="min-w-0 truncate text-sm leading-5"
                                  title={opt.description}
                                >
                                  {opt.label}
                                </CardTitle>
                                <span className="shrink-0 font-mono text-xs leading-5 tabular-nums text-foreground">
                                  {formatOptionNumber(
                                    clampNumber(
                                      Number(draft[opt.key] ?? opt.default),
                                      opt,
                                    ),
                                    opt.step ?? 1,
                                  )}
                                  {opt.unit ? ` ${opt.unit}` : ""}
                                </span>
                              </div>
                              <CardDescription className="h-8 overflow-hidden text-xs leading-4">
                                <span className="line-clamp-2">
                                  {opt.description || "\u00a0"}
                                </span>
                              </CardDescription>
                            </CardHeader>
                            <CardContent className="mt-auto px-3.5">
                              <NumberOptionSlider
                                showValue={false}
                                description={opt.description}
                                min={opt.min}
                                max={opt.max}
                                step={opt.step ?? 1}
                                value={clampNumber(
                                  Number(draft[opt.key] ?? opt.default),
                                  opt,
                                )}
                                onChange={(n) => {
                                  setInstanceDrafts((prev) =>
                                    prev.map((d, i) =>
                                      i === idx ? { ...d, [opt.key]: n } : d,
                                    ),
                                  );
                                }}
                              />
                            </CardContent>
                          </Card>
                        ),
                      )}
                    </div>
                    <div className="flex justify-end">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        disabled={instanceDrafts.length <= 1}
                        onClick={() =>
                          setInstanceDrafts((prev) =>
                            prev.filter((_, i) => i !== idx),
                          )
                        }
                      >
                        Remove
                      </Button>
                    </div>
                  </div>
                ))}
              </>
            )}

            {createMode === "grid" && (
              <>
                <p className="text-xs text-muted-foreground">
                  Multi-select strategy axes. Numbers use a dual-thumb range plus tick.
                  One instance per valid combination.
                </p>
                <div className="grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2">
                  {strategyOptions.map((opt) => {
                    const draft = gridDrafts.find((d) => d.key === opt.key);
                    if (!draft) return null;
                    if (opt.kind === "enum" && draft.kind === "enum") {
                      return (
                        <Card
                          key={opt.key}
                          size="sm"
                          className="h-full min-w-0 gap-3 py-3.5"
                        >
                          <CardHeader className="gap-1.5 px-3.5 pb-0">
                            <CardTitle className="truncate text-sm leading-5">
                              {opt.label}
                            </CardTitle>
                            <CardDescription className="h-8 overflow-hidden text-xs leading-4">
                              <span className="line-clamp-2">
                                {opt.description || "\u00a0"}
                              </span>
                            </CardDescription>
                          </CardHeader>
                          <CardContent className="mt-auto px-3.5">
                            <MultiSelectDropdown
                              label="Values"
                              labelClassName="sr-only"
                              options={opt.values}
                              selected={draft.selected}
                              onChange={(selected) =>
                                setGridDrafts((prev) =>
                                  setEnumAxisSelected(prev, opt.key, selected),
                                )
                              }
                            />
                          </CardContent>
                        </Card>
                      );
                    }
                    if (opt.kind === "number" && draft.kind === "number") {
                      const baseStep = opt.step && opt.step > 0 ? opt.step : 1;
                      const span = Math.max(opt.max - opt.min, baseStep);
                      const tick = Math.min(
                        Math.max(draft.tick > 0 ? draft.tick : baseStep, baseStep),
                        span,
                      );
                      return (
                        <Card
                          key={opt.key}
                          size="sm"
                          className="h-full min-w-0 gap-3 py-3.5"
                        >
                          <CardHeader className="gap-1.5 px-3.5 pb-0">
                            <div className="flex items-baseline justify-between gap-2">
                              <CardTitle
                                className="min-w-0 truncate text-sm leading-5"
                                title={opt.description}
                              >
                                {opt.label}
                              </CardTitle>
                              <span className="shrink-0 font-mono text-xs leading-5 tabular-nums text-foreground">
                                {formatOptionRange(
                                  draft.from,
                                  draft.to,
                                  baseStep,
                                  opt.unit,
                                )}
                              </span>
                            </div>
                            <CardDescription className="m-0 line-clamp-2 text-xs leading-4">
                              {opt.description || "\u00a0"}
                            </CardDescription>
                          </CardHeader>
                          <CardContent className="mt-auto px-3.5">
                            <div className="grid grid-cols-[minmax(0,1fr)_3rem] items-center gap-x-4 gap-y-1">
                              <NumberOptionRangeSlider
                                className="col-start-1 row-start-1"
                                showBounds={false}
                                min={opt.min}
                                max={opt.max}
                                step={baseStep}
                                from={draft.from}
                                to={draft.to}
                                onChange={({ from, to }) => {
                                  const next = clampGridRange(
                                    from,
                                    to,
                                    opt,
                                    from !== draft.from ? "from" : "to",
                                  );
                                  setGridDrafts((prev) =>
                                    prev.map((d) =>
                                      d.key === opt.key && d.kind === "number"
                                        ? { ...d, ...next, tick }
                                        : d,
                                    ),
                                  );
                                }}
                              />
                              <input
                                type="number"
                                className={cn(
                                  fieldInputClass,
                                  "col-start-2 row-start-1 h-7 w-full px-0.5 text-center text-xs tabular-nums",
                                )}
                                min={baseStep}
                                max={span}
                                step={baseStep}
                                value={tick}
                                aria-label={`${opt.label} tick`}
                                title={`Tick (${baseStep}–${span})`}
                                onChange={(e) => {
                                  const raw = Number(e.target.value);
                                  const nextTick = Number.isFinite(raw)
                                    ? Math.min(Math.max(raw, baseStep), span)
                                    : baseStep;
                                  setGridDrafts((prev) =>
                                    prev.map((d) =>
                                      d.key === opt.key && d.kind === "number"
                                        ? { ...d, tick: nextTick }
                                        : d,
                                    ),
                                  );
                                }}
                              />
                              <div className="col-start-1 row-start-2 flex justify-between px-0.5 text-[11px] leading-none text-muted-foreground tabular-nums">
                                <span>{formatOptionNumber(opt.min, baseStep)}</span>
                                <span>{formatOptionNumber(opt.max, baseStep)}</span>
                              </div>
                            </div>
                          </CardContent>
                        </Card>
                      );
                    }
                    return null;
                  })}
                </div>
                <p
                  className={cn(
                    "text-xs",
                    gridPreview.ok ? "text-muted-foreground" : "text-destructive",
                  )}
                >
                  {gridPreview.validCount} valid combination
                  {gridPreview.validCount === 1 ? "" : "s"}
                  {gridPreview.rawCount !== gridPreview.validCount
                    ? ` (${gridPreview.rawCount} raw)`
                    : ""}
                  {gridPreview.error ? ` — ${gridPreview.error}` : ""}
                </p>
              </>
            )}
            </div>
          </div>
          <DialogFooter className="mt-6 border-0 p-0 pt-4">
            <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              type="button"
              disabled={
                saving ||
                (createMode === "grid" && !gridPreview.ok) ||
                (createMode === "manual" &&
                  (instanceDrafts.length === 0 || selectedSymbols.length === 0))
              }
              onClick={() => void handleCreate()}
            >
              {saving
                ? "Creating…"
                : createMode === "grid"
                  ? `Create (${gridPreview.validCount})`
                  : `Create (${manualInstanceCount})`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {detailOpen && selected && (
        <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-clip">
          <h1 className="flex flex-wrap items-center gap-2 text-lg font-semibold tracking-tight">
              {instanceDetailOpen && activeInstance
                ? activeInstance.label
                : selected.name}
              {instanceDetailOpen && activeInstance ? (
                <CatalogStatusBadge status={activeInstance.status} />
              ) : (
                <ExperimentStatusBadge record={selected} />
              )}
          </h1>
          {selected && instanceDetailOpen && activeInstance && (
            <div
              className={cn(
                "flex min-h-0 flex-1 flex-col gap-3",
                activeInstance.status === "completed" && reportRows?.length !== 0
                  ? "overflow-clip"
                  : "overflow-y-auto",
              )}
            >
              {activeInstance.error && (
                <p className="shrink-0 text-sm text-destructive">{activeInstance.error}</p>
              )}
              <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
              {activeInstance.status === "completed" && (
                <ActivityView
                  rows={reportRows ?? []}
                  journals={reportJournals}
                  playbookTitles={reportTitles}
                  live={emptyLive}
                  sectionsDefaultOpen
                  reportPending={reportRows == null}
                  leading={
                    <BacktestPerformancePanel
                      kpi={kpiState}
                      comparison={kpiState.comparison}
                      statistics={kpiState.statistics}
                      outcome={null}
                      loading={kpiState.loading}
                    />
                  }
                  reportApi={{
                    backtestId: selected.id,
                    fromMs: selected.settings.fromMs,
                    toMs: selected.settings.toMs,
                    chartSeriesPath: apiUrl(`/experiments/${selected.id}/instances/${activeInstance.id}/chart-series`),
                  }}
                  chartsHold={
                    (activeInstance.instanceReport?.statistics?.monthlyReturns?.length ?? 0) === 0 &&
                    kpiState.loading
                  }
                  chartsExtra={(() => {
                    const fromReport = activeInstance.instanceReport?.statistics?.monthlyReturns;
                    const monthly =
                      fromReport && fromReport.length > 0
                        ? fromReport
                        : (kpiState.statistics?.monthlyReturns ?? []);
                    return monthly.length > 0 ? (
                      <MonthlyReturnChart series={monthly} height={monthlyChartHeight} embedded fill />
                    ) : null;
                  })()}
                  className="min-h-0 flex-1"
                />
              )}
              {activeInstance.status === "failed" && (
                <p className="text-sm text-destructive">
                  Instance failed
                  {activeInstance.error ? `: ${activeInstance.error}` : ""}
                </p>
              )}
              {selected.status === "draft" && (
                <p className="text-sm text-muted-foreground">
                  Draft — run the experiment to simulate this instance.
                </p>
              )}
              {selected.status === "running" && activeInstance.status !== "completed" && (
                <p className="text-sm text-muted-foreground">Experiment is still running.</p>
              )}
              </div>
            </div>
          )}
          {selected && !instanceDetailOpen && (
            <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
              <p className="text-sm text-muted-foreground">
                {formatBacktestRange(selected.settings.fromMs, selected.settings.toMs)}
                {" · "}
                {instanceTotal || selected.progressInstanceTotal} instance
                {(instanceTotal || selected.progressInstanceTotal) === 1 ? "" : "s"}
                {selected.progressStageLabel
                  ? ` · ${selected.progressStageLabel}`
                  : ""}
              </p>
              {selected.error && <p className="text-sm text-destructive">{selected.error}</p>}
              {selected.status === "materializing" && (
                <p className="text-sm text-muted-foreground">
                  Expanding grid into instances
                  {selected.progressInstanceTotal > 0
                    ? ` · ${selected.progressInstanceDone}/${selected.progressInstanceTotal}`
                    : "…"}
                </p>
              )}
              {selected.status === "draft" && (
                <p className="text-sm text-muted-foreground">
                  Draft — run to pre-ingest, build shared KPI once, and simulate all instances.
                </p>
              )}
              {experimentHasResults(selected.status) &&
                instanceTotal > 0 &&
                instanceRows.length >= instanceTotal && (
                  <>
                    <BacktestPerformancePanel
                      title="Win rate"
                      kpi={experimentPerf}
                      comparison={undefined}
                      statistics={experimentPerf.statistics}
                      outcome={null}
                      loading={experimentPerf.loading}
                    />
                    <ExperimentPerformanceMetrics
                      experiment={{ ...selected, instances: instanceRows }}
                    />
                  </>
                )}
              <details className="relative self-end text-sm">
                <summary className="cursor-pointer rounded-md border px-2 py-1 text-muted-foreground">
                  Columns ({shownInstanceColumns.length}/{instanceColumns.length})
                </summary>
                <div className="absolute right-0 z-10 mt-1 flex w-56 flex-col gap-1 rounded-md border bg-background p-2 shadow">
                  {instanceColumns.map((c) => (
                    <label key={c.id} className="flex items-center gap-2 text-xs">
                      <input
                        type="checkbox"
                        checked={visibleInstanceColumns.includes(c.id)}
                        onChange={() => toggleInstanceColumn(c.id)}
                      />
                      {c.header}
                    </label>
                  ))}
                  <button
                    type="button"
                    className="mt-1 text-left text-xs underline"
                    onClick={() => setVisibleInstanceColumns(DEFAULT_VISIBLE_INSTANCE_COLUMNS)}
                  >
                    Reset to default
                  </button>
                </div>
              </details>
              <FilterableTable
                rows={instanceRows}
                columns={shownInstanceColumns}
                rowKey={(r) => r.id}
                emptyMessage={
                  selected.status === "materializing"
                    ? "Materializing instances…"
                    : "No instances."
                }
                className="min-h-0 flex-1"
                onRowClick={(inst) => void openInstanceDetail(inst)}
                serverDriven={{
                  sortColumnId: instanceSort,
                  sortDir: instanceOrder,
                  onSortChange: (col, dir) => {
                    setInstanceSort(col);
                    setInstanceOrder(dir);
                    setInstanceOffset(0);
                  },
                  q: instanceQ,
                  onQChange: (q) => {
                    setInstanceQ(q);
                    setInstanceOffset(0);
                  },
                  total: instanceTotal,
                  pageOffset: instanceOffset,
                  pageLimit: INSTANCE_PAGE_SIZE,
                  onPageChange: setInstanceOffset,
                }}
              />
            </div>
          )}
          {!instanceDetailOpen && (
          <div className="flex flex-col gap-2 border-t pt-3">
            {(selected.status === "running" || selected.status === "materializing") && (
              <p className="text-sm text-muted-foreground">
                {selected.status === "materializing" ? "Materializing" : "Running"}
                {experimentProgressPercent(selected) != null
                  ? ` · ${experimentProgressPercent(selected)}%`
                  : ""}
                {selected.progressInstanceTotal > 0
                  ? ` · ${selected.progressInstanceDone}/${selected.progressInstanceTotal}`
                  : ""}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              {selected.status === "running" && (
                <Button
                  type="button"
                  variant="outline"
                  disabled={cancelling || deleting}
                  onClick={() => void handleCancel()}
                >
                  {cancelling ? "Cancelling…" : "Cancel run"}
                </Button>
              )}
              {selected.status !== "running" && selected.status !== "materializing" && (
                <Button type="button" disabled={running || deleting} onClick={() => void handleRun()}>
                  {running ? "Starting…" : "Run experiment"}
                </Button>
              )}
              {selected.status !== "running" && failedInstanceCount > 0 && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={running || deleting}
                    onClick={() => void handleRun({ retryFailed: true })}
                  >
                    {running ? "Starting…" : `Retry failed (${failedInstanceCount})`}
                  </Button>
                )}
              <Button
                type="button"
                variant="destructive"
                disabled={deleting || cancelling}
                onClick={() => requestDelete()}
              >
                {deleting ? "Deleting…" : "Delete"}
              </Button>
            </div>
          </div>
          )}
        </div>
      )}

      <Dialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <DialogContent className="p-6 sm:max-w-md">
          <DialogHeader className="border-0 p-0 pb-4">
            <DialogTitle>Delete running experiment?</DialogTitle>
            <DialogDescription>
              This will cancel the active run, then permanently delete the experiment and all
              instance workspaces and KPI data. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="mt-4 border-0 p-0">
            <Button type="button" variant="outline" onClick={() => setDeleteConfirmOpen(false)}>
              Keep
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={deleting}
              onClick={() => void handleDelete(true)}
            >
              {deleting ? "Deleting…" : "Cancel run and delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
