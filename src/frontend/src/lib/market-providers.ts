/** Market-data providers for the lake (ingest / parquet partition key). */

import { PLUGGED_PROVIDERS } from "@/lib/provider-registry";

export const MARKET_PROVIDERS = PLUGGED_PROVIDERS.map((p) => ({
  id: p.id,
  label: p.label,
})) as ReadonlyArray<{ id: string; label: string }>;

export type MarketProviderId = string;

export const DEFAULT_MARKET_PROVIDER: MarketProviderId = "binance";

export function isMarketProviderId(id: string): boolean {
  return MARKET_PROVIDERS.some((p) => p.id === id);
}

/** Empty or unknown → default (legacy catalog rows). */
export function normalizeMarketProvider(id: string | undefined | null): MarketProviderId {
  const trimmed = (id ?? "").trim();
  if (isMarketProviderId(trimmed)) return trimmed;
  return DEFAULT_MARKET_PROVIDER;
}

export function marketProviderLabel(id: string | undefined | null): string {
  const normalized = normalizeMarketProvider(id);
  return MARKET_PROVIDERS.find((p) => p.id === normalized)?.label ?? normalized;
}
