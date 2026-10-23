"use client";

import { BacktestDatePicker } from "@/components/backtest-date-picker";
import {
  backtestDateInputToFromMs,
  backtestDateInputToToMs,
  backtestFromMsToDateInput,
  backtestToMsToDateInput,
  formatBacktestRange,
  normalizePlaybookSlug,
  type BacktestSettings,
  type PlaybookOption,
} from "@/lib/backtest-shared";

const fieldInputClass =
  "w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50";

type Props = {
  name: string;
  description?: string;
  settings: BacktestSettings;
  playbookOptions: PlaybookOption[];
  onNameChange: (name: string) => void;
  onDescriptionChange?: (description: string) => void;
  onSettingsChange: (patch: Partial<BacktestSettings>) => void;
  showDescription?: boolean;
};

export function BacktestSettingsForm({
  name,
  description = "",
  settings,
  playbookOptions,
  onNameChange,
  onDescriptionChange,
  onSettingsChange,
  showDescription = false,
}: Props) {
  const playbookSlug = normalizePlaybookSlug(settings.playbook);
  const selectOptions = [...playbookOptions];
  if (playbookSlug && !selectOptions.some((o) => o.id === playbookSlug)) {
    selectOptions.push({ id: playbookSlug, label: playbookSlug });
  }

  const fromDate = backtestFromMsToDateInput(settings.fromMs);
  const toDate = backtestToMsToDateInput(settings.toMs);

  return (
    <div className="grid gap-3 text-sm">
      <label className="grid gap-1">
        <span className="text-muted-foreground">Name</span>
        <input className={fieldInputClass} value={name} onChange={(e) => onNameChange(e.target.value)} />
      </label>
      {showDescription && onDescriptionChange && (
        <label className="grid gap-1">
          <span className="text-muted-foreground">Description</span>
          <textarea
            className={fieldInputClass}
            rows={2}
            value={description}
            onChange={(e) => onDescriptionChange(e.target.value)}
          />
        </label>
      )}
      <div className="grid grid-cols-2 gap-3">
        <label className="grid gap-1">
          <span className="text-muted-foreground">Symbol</span>
          <input
            className={fieldInputClass}
            value={settings.symbol}
            onChange={(e) => onSettingsChange({ symbol: e.target.value })}
          />
        </label>
        <label className="grid gap-1">
          <span className="text-muted-foreground">Playbook</span>
          <select
            className={fieldInputClass}
            value={playbookSlug}
            onChange={(e) => onSettingsChange({ playbook: e.target.value })}
          >
            {selectOptions.map((opt) => (
              <option key={opt.id} value={opt.id}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <BacktestDatePicker
          id="backtest-from"
          label="From (UTC)"
          value={fromDate}
          max={toDate || undefined}
          onChange={(dateInput) => {
            const fromMs = backtestDateInputToFromMs(dateInput);
            if (fromMs == null) return;
            const patch: Partial<BacktestSettings> = { fromMs };
            if (fromMs >= settings.toMs) {
              const bumped = backtestDateInputToToMs(dateInput);
              if (bumped != null && bumped > fromMs) patch.toMs = bumped;
            }
            onSettingsChange(patch);
          }}
        />
        <BacktestDatePicker
          id="backtest-to"
          label="To (UTC, inclusive)"
          value={toDate}
          min={fromDate || undefined}
          onChange={(dateInput) => {
            const toMs = backtestDateInputToToMs(dateInput);
            if (toMs == null || toMs <= settings.fromMs) return;
            onSettingsChange({ toMs });
          }}
        />
      </div>
      <p className="text-xs text-muted-foreground">{formatBacktestRange(settings.fromMs, settings.toMs)}</p>
      <p className="text-xs text-muted-foreground">
        Defaults assume Bybit EU spot: fund USDC only for BTC/ETH, ~95% target when long, and modest taker-style
        costs. Runs download missing candles for the chosen range; enable force re-download to replace existing parquet.
      </p>
      <div className="grid grid-cols-2 gap-3">
        {(
          [
            ["Initial USDC", "initialUsdc", settings.initialUsdc],
            ["Initial USDT", "initialUsdt", settings.initialUsdt],
            ["Initial base", "initialBase", settings.initialBase],
            ["Target weight %", "targetWeightPct", settings.targetWeightPct],
            ["Cash reserve (quote)", "cashReserveQuote", settings.cashReserveQuote],
            ["Min order (quote)", "minOrderQuote", settings.minOrderQuote],
            ["Spread (bps, full width)", "spreadBps", settings.spreadBps ?? 0],
            ["Slippage (bps)", "slippageBps", settings.slippageBps],
            ["Fee (bps)", "feeBps", settings.feeBps],
          ] as const
        ).map(([label, key, val]) => (
          <label key={key} className="grid gap-1">
            <span className="text-muted-foreground">{label}</span>
            <input
              type="number"
              className={fieldInputClass}
              value={val}
              onChange={(e) => onSettingsChange({ [key]: Number(e.target.value) } as Partial<BacktestSettings>)}
            />
          </label>
        ))}
      </div>
      <label className="flex items-center gap-2 text-xs">
        <input
          type="checkbox"
          checked={settings.redownloadMissing}
          onChange={(e) => onSettingsChange({ redownloadMissing: e.target.checked })}
        />
        Force re-download for the range (replaces existing hourly candles; leave off to only fill gaps)
      </label>
    </div>
  );
}
