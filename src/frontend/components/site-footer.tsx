"use client";

import { ThemeToggle } from "@/components/theme-toggle";

export function SiteFooter() {
  return (
    <footer className="shrink-0 border-t bg-background">
      <div className="flex w-full items-center justify-between gap-4 px-4 py-2 text-xs text-muted-foreground lg:px-6">
        <span>Local stack</span>
        <div className="flex items-center gap-3">
          <span>Dagobert</span>
          <ThemeToggle />
        </div>
      </div>
    </footer>
  );
}
