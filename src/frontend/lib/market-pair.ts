export type MarketPair = {
  base: string;
  quote: string;
};

export function pipelineSymbol(base: string, quote: string): string {
  return `${base.trim().toUpperCase()}${quote.trim().toUpperCase()}`;
}

/** Mirror Go shared.SplitPipelineSymbol for common quotes. */
const PIPELINE_QUOTES = [
  "USDT",
  "USDC",
  "BUSD",
  "FDUSD",
  "TUSD",
  "BTC",
  "ETH",
  "BNB",
  "EUR",
  "TRY",
  "BRL",
  "GBP",
  "AUD",
  "USD",
] as const;

export function splitPipelineSymbol(sym: string): MarketPair | null {
  const s = sym.trim().toUpperCase();
  for (const q of PIPELINE_QUOTES) {
    if (s.endsWith(q) && s.length > q.length) {
      return { base: s.slice(0, -q.length), quote: q };
    }
  }
  return null;
}

export function formatMarketPair(p: MarketPair): string {
  return `${p.base} · ${p.quote}`;
}

export function pairKey(p: MarketPair): string {
  return `${p.base}/${p.quote}`;
}

export function pairsEqual(a: MarketPair, b: MarketPair): boolean {
  return a.base === b.base && a.quote === b.quote;
}
