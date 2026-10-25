export type ChartIndicatorMeta = {
  id: string;
  label: string;
  unit: string;
  pane: "price" | "oscillator";
  style: "line" | "histogram";
  reference?: number[];
};

/** One indicator column, aligned with ChartColumns.timeSec. Missing samples are NaN. */
export type ChartColumnIndicator = ChartIndicatorMeta & {
  value: number[];
};

/** Plot columns for one instance. Times are unix seconds. */
export type ChartColumns = {
  timeSec: number[];
  open: number[];
  high: number[];
  low: number[];
  close: number[];
  indicators: ChartColumnIndicator[];
  portfolio: { time: number; value: number }[];
  markers: { time: number; side: "buy" | "sell" }[];
};

export const emptyChartColumns: ChartColumns = {
  timeSec: [],
  open: [],
  high: [],
  low: [],
  close: [],
  indicators: [],
  portfolio: [],
  markers: [],
};
