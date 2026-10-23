"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ThemeToggle } from "@/components/theme-toggle";
import { cn } from "@/lib/utils";

const links = [
  { href: "/", label: "Portfolio" },
  { href: "/strategy", label: "Strategy" },
  { href: "/backtesting", label: "Backtesting" },
  { href: "/data", label: "Data" },
  { href: "/processing", label: "Processing" },
  { href: "/wiki", label: "Wiki" },
];

export function SiteHeader() {
  const pathname = usePathname();

  return (
    <header className="shrink-0 border-b bg-background">
      <div className="flex w-full items-center justify-between gap-4 px-4 py-3 lg:px-6">
        <div className="flex min-w-0 items-center gap-6 sm:gap-8">
          <span className="shrink-0 font-semibold tracking-tight">Trading Copilot</span>
          <nav className="flex flex-wrap gap-3 text-sm sm:gap-4">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={cn(
                  "transition-colors",
                  pathname === link.href || (link.href !== "/" && pathname.startsWith(link.href))
                    ? "text-foreground font-medium"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
        <ThemeToggle />
      </div>
    </header>
  );
}
