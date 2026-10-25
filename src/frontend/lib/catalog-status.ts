/** Badge className for catalog status chips (experiments, instances, backtests, providers). */
export function catalogStatusBadgeClass(status: string): string {
  switch (status) {
    case "completed":
    case "ready":
      return "border-transparent bg-emerald-600/15 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400";
    case "partial":
      return "border-transparent bg-yellow-500/25 text-yellow-800 dark:bg-yellow-500/20 dark:text-yellow-300";
    case "cancelled":
      return "border-transparent bg-amber-500/20 text-amber-800 dark:bg-amber-500/25 dark:text-amber-300";
    case "failed":
      return "border-transparent bg-red-600/15 text-red-700 dark:bg-red-500/20 dark:text-red-400";
    case "running":
    case "materializing":
    case "setting_up":
      return "border-transparent bg-sky-600/15 text-sky-700 dark:bg-sky-500/20 dark:text-sky-400";
    default:
      return "";
  }
}

export function catalogStatusBadgeVariant(
  status: string,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "completed":
    case "ready":
    case "partial":
    case "cancelled":
    case "failed":
    case "running":
    case "materializing":
    case "setting_up":
      return "outline";
    default:
      return "secondary";
  }
}

/** Experiment finished with at least some successful instances. */
export function experimentHasResults(status: string): boolean {
  return status === "completed" || status === "partial";
}
