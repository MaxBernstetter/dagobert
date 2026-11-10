import { listProviderPairs } from "@/lib/provider-service";
import {
  DEFAULT_MARKET_PROVIDER,
  normalizeMarketProvider,
} from "@/lib/market-providers";
import type { MarketPair } from "@/lib/market-pair";

const memory = new Map<string, MarketPair[]>();
const inflight = new Map<string, Promise<MarketPair[]>>();

function normalizePairs(raw: unknown): MarketPair[] {
  if (!Array.isArray(raw)) return [];
  const out: MarketPair[] = [];
  for (const row of raw) {
    if (!row || typeof row !== "object") continue;
    const base = String((row as MarketPair).base ?? "")
      .trim()
      .toUpperCase();
    const quote = String((row as MarketPair).quote ?? "")
      .trim()
      .toUpperCase();
    if (!base || !quote || base === quote) continue;
    out.push({ base, quote });
  }
  return out;
}

/**
 * Load markets for a provider: the stored setup snapshot when ready (ProviderService.ListProviderPairs).
 */
export function loadProviderPairs(provider: string): Promise<MarketPair[]> {
  const id = normalizeMarketProvider(provider);
  const cached = memory.get(id);
  if (cached) return Promise.resolve(cached);

  const existing = inflight.get(id);
  if (existing) return existing;

  const pending = (async () => {
    const pairs = normalizePairs(await listProviderPairs(id));
    memory.set(id, pairs);
    return pairs;
  })().finally(() => {
    inflight.delete(id);
  });

  inflight.set(id, pending);
  return pending;
}

/** Drop session cache after Connectors activate/refresh. */
export function invalidateProviderPairs(provider?: string): void {
  if (provider) {
    memory.delete(normalizeMarketProvider(provider));
    return;
  }
  memory.clear();
}

/** Fire-and-forget warm for ready providers. */
export function prefetchProviderPairs(
  provider: string = DEFAULT_MARKET_PROVIDER,
): void {
  void loadProviderPairs(provider).catch(() => {
    /* background warm; UI surfaces errors on interactive load */
  });
}
