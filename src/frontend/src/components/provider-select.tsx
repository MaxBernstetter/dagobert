
import Link from "@/components/link";
import { useEffect, useState } from "react";

import {
  DEFAULT_MARKET_PROVIDER,
  normalizeMarketProvider,
  type MarketProviderId,
} from "@/lib/market-providers";
import { fetchReadyProviders, type ReadyProvider } from "@/lib/ready-providers";
import { cn } from "@/lib/utils";

const fieldInputClass =
  "w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50";

type Props = {
  value?: string;
  onChange: (provider: MarketProviderId) => void;
  id?: string;
  className?: string;
  disabled?: boolean;
  /** When true, omit the outer label (caller supplies one). */
  bare?: boolean;
};

export function ProviderSelect({
  value,
  onChange,
  id = "market-provider",
  className,
  disabled,
  bare = false,
}: Props) {
  const [ready, setReady] = useState<ReadyProvider[] | null>(null);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await fetchReadyProviders();
        if (cancelled) return;
        setReady(list);
        setLoadError("");
        if (list.length === 0) return;
        const current = normalizeMarketProvider(value || DEFAULT_MARKET_PROVIDER);
        if (!list.some((p) => p.id === current)) {
          onChange(list[0].id);
        }
      } catch (err) {
        if (!cancelled) {
          setReady([]);
          setLoadError(err instanceof Error ? err.message : String(err));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
    // Intentionally once on mount; parent owns subsequent value.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const options = ready ?? [];
  const noneReady = ready !== null && options.length === 0;
  const selected =
    options.find((p) => p.id === value)?.id ??
    options[0]?.id ??
    normalizeMarketProvider(value || DEFAULT_MARKET_PROVIDER);

  const select = (
    <select
      id={id}
      className={cn(className ?? fieldInputClass, noneReady && "opacity-60")}
      value={selected}
      disabled={disabled || noneReady || ready === null}
      onChange={(e) => onChange(e.target.value)}
    >
      {ready === null ? (
        <option value="">Loading…</option>
      ) : noneReady ? (
        <option value="">No ready providers</option>
      ) : (
        options.map((p) => (
          <option key={p.id} value={p.id}>
            {p.label}
          </option>
        ))
      )}
    </select>
  );

  const hint = noneReady ? (
    <span className="text-xs text-muted-foreground">
      Activate a provider under{" "}
      <Link href="/connectors" className="underline underline-offset-2">
        Connectors
      </Link>
      {loadError ? ` (${loadError})` : ""}.
    </span>
  ) : null;

  if (bare) {
    return (
      <div className="grid gap-1">
        {select}
        {hint}
      </div>
    );
  }
  return (
    <label className="grid gap-1">
      <span className="text-muted-foreground">Provider</span>
      {select}
      {hint}
    </label>
  );
}
