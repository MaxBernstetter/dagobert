"use client";

import { useMemo } from "react";

import { MultiSelectDropdown } from "@/components/multi-select-dropdown";
import {
  formatMarketPair,
  pairKey,
  pipelineSymbol,
  type MarketPair,
} from "@/lib/market-pair";
import { pipelineSymbolsToPairs } from "@/lib/symbol-picker";
import { cn } from "@/lib/utils";

type Props = {
  pairs: MarketPair[];
  value: MarketPair[];
  onChange: (pairs: MarketPair[]) => void;
  label?: string;
  disabled?: boolean;
  loading?: boolean;
  emptyHint?: string;
};

function PairTag({ pair }: { pair: MarketPair }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded border px-1 py-px font-mono text-[10px] leading-4",
        "border-emerald-500/40 bg-emerald-600/15 text-emerald-700",
        "dark:border-emerald-500/50 dark:bg-emerald-500/20 dark:text-emerald-400",
      )}
    >
      {pair.base}
      <span className="text-emerald-700/70 dark:text-emerald-400/70">·</span>
      {pair.quote}
    </span>
  );
}

/** Multi-select of markets from the activated provider catalog (Connectors → Ingestion). */
export function SymbolPicker({
  pairs,
  value,
  onChange,
  label = "Markets",
  disabled,
  loading,
  emptyHint = "Activate a provider under Connectors to load markets",
}: Props) {
  const options = useMemo(
    () =>
      pairs.map((p) => ({
        value: pipelineSymbol(p.base, p.quote),
        label: formatMarketPair(p),
      })),
    [pairs],
  );

  const selected = useMemo(
    () => value.map((p) => pipelineSymbol(p.base, p.quote)),
    [value],
  );

  return (
    <div className="grid gap-1.5">
      <MultiSelectDropdown
        label={label}
        options={options}
        selected={selected}
        onChange={(next) => {
          onChange(pipelineSymbolsToPairs(next.map(String), pairs));
        }}
        searchable
        preview="count"
        countUnit="pairs"
        disabled={disabled || loading}
        emptyHint={loading ? "Loading markets…" : emptyHint}
        triggerClassName="truncate text-left"
      />
      {value.length > 0 && (
        <div className="flex flex-wrap gap-2.5">
          {value.map((p) => (
            <PairTag key={pairKey(p)} pair={p} />
          ))}
        </div>
      )}
    </div>
  );
}
