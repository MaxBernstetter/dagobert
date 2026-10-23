"use client";

import { CircleHelp } from "lucide-react";
import type { ReactNode } from "react";

export function HelpTip({
  label,
  tip,
  children,
}: {
  label: string;
  tip: string;
  children?: ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-1">
      {children ?? <span>{label}</span>}
      <span className="relative inline-flex group/help">
        <CircleHelp
          className="size-3.5 shrink-0 text-muted-foreground cursor-help"
          aria-label={`Help: ${label}`}
        />
        <span
          role="tooltip"
          className="pointer-events-none absolute left-0 top-full z-50 mt-1 hidden w-64 rounded-md border bg-popover px-2.5 py-2 text-xs font-normal leading-snug text-popover-foreground shadow-md group-hover/help:block group-focus-within/help:block"
        >
          {tip}
        </span>
      </span>
    </span>
  );
}
