import { ChevronRight } from "lucide-react";
import { type ReactNode } from "react";
import { kpiTileShellClassName, toneTextClassName, type KpiTone } from "@/lib/strategy-kpi-tone";
import { cn } from "@/lib/utils";

export function BacktestDetailSection({
  title,
  children,
  className,
  defaultOpen = true,
}: {
  title: string;
  children: ReactNode;
  className?: string;
  defaultOpen?: boolean;
}) {
  return (
    <details
      open={defaultOpen}
      className={cn("group rounded-lg border bg-muted/20 text-sm", className)}
    >
      <summary
        className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 font-medium [&::-webkit-details-marker]:hidden"
      >
        <ChevronRight
          className="size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90"
          aria-hidden
        />
        <span>{title}</span>
      </summary>
      <div className="space-y-3 border-t border-border/50 px-4 pb-3 pt-2">{children}</div>
    </details>
  );
}

export function BacktestMetricGroup({
  title,
  hint,
  children,
  className,
}: {
  title: string;
  hint?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("rounded-md border border-border/50 bg-muted/15 px-3 py-2", className)}>
      <p className="text-xs font-medium">{title}</p>
      {hint ? <p className="text-[0.65rem] text-muted-foreground mt-0.5 mb-1">{hint}</p> : null}
      <div className="divide-y divide-border/40">{children}</div>
    </div>
  );
}

export function BacktestMetricRow({
  label,
  value,
  tone = "neutral",
  title,
}: {
  label: string;
  value: ReactNode;
  tone?: KpiTone;
  title?: string;
}) {
  const valueColor =
    tone === "good" || tone === "bad" ? toneTextClassName(tone) : "text-foreground";
  return (
    <div
      className="flex items-baseline justify-between gap-4 py-1.5 text-sm"
      title={title}
    >
      <span className="text-muted-foreground min-w-0">{label}</span>
      <span className={cn("shrink-0 text-right font-medium tabular-nums", valueColor)}>
        {value}
      </span>
    </div>
  );
}

export function BacktestMetricTile({
  label,
  value,
  tone = "neutral",
  className,
  title,
  valueClassName,
}: {
  label: string;
  value: ReactNode;
  tone?: KpiTone;
  className?: string;
  title?: string;
  valueClassName?: string;
}) {
  const accent = tone === "good" || tone === "bad" ? toneTextClassName(tone) : null;
  return (
    <div
      className={cn("rounded-md border min-w-[5.5rem] px-2.5 py-1.5", kpiTileShellClassName, className)}
      title={title}
    >
      <p
        className={cn(
          "text-[0.65rem] uppercase tracking-wide",
          accent ?? "text-muted-foreground",
        )}
      >
        {label}
      </p>
      <p className={cn("font-semibold tabular-nums", accent ?? "text-foreground", valueClassName)}>
        {value}
      </p>
    </div>
  );
}
