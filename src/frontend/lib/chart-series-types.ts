export type ChartIndicatorMeta = { id: string; label: string; color: string };

export type ChartBarPayload = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  indicators: Record<string, number | null>;
};
