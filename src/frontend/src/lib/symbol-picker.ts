import {
  pairKey,
  pairsEqual,
  pipelineSymbol,
  type MarketPair,
} from "@/lib/market-pair";

const PREFERRED_BASES = ["BTC", "ETH"] as const;
const PREFERRED_QUOTES = ["USDT", "USDC"] as const;

function uniqSorted(values: string[]): string[] {
  return [...new Set(values.map((v) => v.trim().toUpperCase()).filter(Boolean))].sort();
}

export function uniqueBases(pairs: MarketPair[]): string[] {
  return uniqSorted(pairs.map((p) => p.base));
}

export function uniqueQuotes(pairs: MarketPair[]): string[] {
  return uniqSorted(pairs.map((p) => p.quote));
}

/** Bases/quotes present in the current selection (for hydrating the two lists). */
export function basesQuotesFromPairs(selected: MarketPair[]): {
  bases: string[];
  quotes: string[];
} {
  return {
    bases: uniqueBases(selected),
    quotes: uniqueQuotes(selected),
  };
}

function sortPairs(out: MarketPair[]): MarketPair[] {
  out.sort((a, b) => {
    if (a.base !== b.base) return a.base.localeCompare(b.base);
    return a.quote.localeCompare(b.quote);
  });
  return out;
}

/** Full cartesian of bases × quotes (base ≠ quote), no catalog filter. */
export function cartesianBaseQuotePairs(
  bases: string[],
  quotes: string[],
): MarketPair[] {
  const bs = uniqSorted(bases);
  const qs = uniqSorted(quotes);
  if (bs.length === 0 || qs.length === 0) return [];
  const out: MarketPair[] = [];
  for (const base of bs) {
    for (const quote of qs) {
      if (base === quote) continue;
      out.push({ base, quote });
    }
  }
  return sortPairs(out);
}

/**
 * Cartesian of selected bases × quotes, intersected with the provider catalog
 * so invented markets are never returned.
 */
export function expandBaseQuoteSelection(
  bases: string[],
  quotes: string[],
  catalog: MarketPair[],
): MarketPair[] {
  const baseSet = new Set(uniqSorted(bases));
  const quoteSet = new Set(uniqSorted(quotes));
  if (baseSet.size === 0 || quoteSet.size === 0) return [];
  const out: MarketPair[] = [];
  const seen = new Set<string>();
  for (const p of catalog) {
    const base = p.base.trim().toUpperCase();
    const quote = p.quote.trim().toUpperCase();
    if (!baseSet.has(base) || !quoteSet.has(quote)) continue;
    const key = pairKey({ base, quote });
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ base, quote });
  }
  return sortPairs(out);
}

/** Cartesian cells that are not offered by the provider catalog. */
export function unavailableBaseQuotePairs(
  bases: string[],
  quotes: string[],
  catalog: MarketPair[],
): MarketPair[] {
  const available = new Set(
    expandBaseQuoteSelection(bases, quotes, catalog).map((p) => pairKey(p)),
  );
  return cartesianBaseQuotePairs(bases, quotes).filter(
    (p) => !available.has(pairKey(p)),
  );
}

/** Prefer BTC/ETH × USDT(/USDC) when those catalog pairs exist; else a small fallback. */
export function defaultSymbolPairs(catalog: MarketPair[]): MarketPair[] {
  if (catalog.length === 0) {
    return [
      { base: "BTC", quote: "USDT" },
      { base: "ETH", quote: "USDT" },
    ];
  }
  const bases = PREFERRED_BASES.filter((b) =>
    catalog.some((p) => p.base.toUpperCase() === b),
  );
  const quotes = PREFERRED_QUOTES.filter((q) =>
    catalog.some((p) => p.quote.toUpperCase() === q),
  );
  const preferredBases = bases.length > 0 ? [...bases] : uniqueBases(catalog).slice(0, 2);
  const preferredQuotes =
    quotes.length > 0 ? [quotes[0]!] : uniqueQuotes(catalog).slice(0, 1);
  const expanded = expandBaseQuoteSelection(preferredBases, preferredQuotes, catalog);
  if (expanded.length > 0) return expanded;
  return catalog.slice(0, 2).map((p) => ({
    base: p.base.toUpperCase(),
    quote: p.quote.toUpperCase(),
  }));
}

export function pairsToPipelineSymbols(pairs: MarketPair[]): string[] {
  return pairs.map((p) => pipelineSymbol(p.base, p.quote));
}

export function pipelineSymbolsToPairs(
  symbols: string[],
  catalog: MarketPair[],
): MarketPair[] {
  const byPipe = new Map(
    catalog.map((p) => [pipelineSymbol(p.base, p.quote), p] as const),
  );
  const out: MarketPair[] = [];
  const seen = new Set<string>();
  for (const sym of symbols) {
    const hit = byPipe.get(sym.trim().toUpperCase());
    if (!hit) continue;
    const key = pairKey(hit);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ base: hit.base.toUpperCase(), quote: hit.quote.toUpperCase() });
  }
  return out;
}

export function samePairSet(a: MarketPair[], b: MarketPair[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((p) => b.some((q) => pairsEqual(p, q)));
}
