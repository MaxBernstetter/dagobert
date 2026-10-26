/** Shared look for the ECharts panes. Reads the document theme at call time. */

export type ChartTheme = {
  text: string;
  grid: string;
  axisLine: string;
  candleUp: string;
  candleDown: string;
  candleUpFill: string;
  candleDownFill: string;
  portfolio: string;
  hold: string;
  buy: string;
  sell: string;
  highlight: string;
  positive: string;
  negative: string;
  indicators: string[];
};

const LIGHT: ChartTheme = {
  text: "#71717a",
  grid: "rgba(228, 228, 231, 0.9)",
  axisLine: "#d4d4d8",
  candleUp: "#3f6212",
  candleDown: "#be123c",
  candleUpFill: "rgba(63, 98, 18, 0.22)",
  candleDownFill: "rgba(190, 18, 60, 0.16)",
  portfolio: "#0369a1",
  hold: "#71717a",
  buy: "#059669",
  sell: "#e11d48",
  highlight: "rgba(2, 132, 199, 0.14)",
  positive: "rgba(5, 150, 105, 0.75)",
  negative: "rgba(225, 29, 72, 0.7)",
  indicators: ["#0284c7", "#7c3aed", "#d97706", "#0f766e"],
};

const DARK: ChartTheme = {
  text: "#a1a1aa",
  grid: "rgba(63, 63, 70, 0.55)",
  axisLine: "#3f3f46",
  candleUp: "#a3e635",
  candleDown: "#fb7185",
  candleUpFill: "rgba(163, 230, 53, 0.22)",
  candleDownFill: "rgba(251, 113, 133, 0.18)",
  portfolio: "#7dd3fc",
  hold: "#a1a1aa",
  buy: "#34d399",
  sell: "#fb7185",
  highlight: "rgba(125, 211, 252, 0.16)",
  positive: "rgba(52, 211, 153, 0.75)",
  negative: "rgba(251, 113, 133, 0.72)",
  indicators: ["#38bdf8", "#c4b5fd", "#fbbf24", "#5eead4"],
};

export const AXIS_TITLE_MIN_PX = 120;

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function readChartTheme(): ChartTheme {
  return document.documentElement.classList.contains("dark") ? DARK : LIGHT;
}

export function indicatorColor(index: number, theme: ChartTheme = readChartTheme()): string {
  return theme.indicators[index % theme.indicators.length];
}

export function chartFontFamily(): string {
  return getComputedStyle(document.body).fontFamily || "sans-serif";
}

export function formatUtcChartTime(sec: number): string {
  const d = new Date(sec * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())} UTC`;
}

/** Tick text that matches how much time is on screen. `ms` is a UTC timestamp. */
export function formatUtcTick(ms: number, spanSec: number): string {
  const d = new Date(ms);
  const pad = (n: number) => String(n).padStart(2, "0");
  if (spanSec <= 2 * 86400) return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
  if (spanSec <= 400 * 86400) return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}`;
  return `${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
}

export function formatAxisNumber(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 1000) return value.toLocaleString(undefined, { maximumFractionDigits: 0 });
  if (abs >= 10) return value.toLocaleString(undefined, { maximumFractionDigits: 1 });
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

export function formatUsdc(value: number): string {
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 0 })} USDC`;
}
