import { Badge } from "@/components/ui/badge";
import { backtestStatusLabel } from "@/lib/backtest-shared";

export function BacktestStatusBadge({
  status,
  progressDone,
  progressTotal,
  progressStage,
  progressStages,
  progressStageLabel,
}: {
  status: string;
  progressDone: number;
  progressTotal: number;
  progressStage: number;
  progressStages: number;
  progressStageLabel: string;
}) {
  const label = backtestStatusLabel({
    status,
    progressDone,
    progressTotal,
    progressStage,
    progressStages,
    progressStageLabel,
  });
  const variant =
    status === "completed"
      ? "default"
      : status === "failed"
        ? "destructive"
        : status === "running"
          ? "outline"
          : "secondary";
  return <Badge variant={variant}>{label}</Badge>;
}
