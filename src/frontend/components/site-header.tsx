"use client";

import { Wrench } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

const links = [
  { href: "/experiments", label: "Experiment Suite" },
  { href: "/wiki", label: "Wiki" },
];

export function SiteHeader() {
  const pathname = usePathname();
  const [toolsOpen, setToolsOpen] = useState(false);

  return (
    <header className="shrink-0 border-b bg-background">
      <div className="flex w-full items-center justify-between gap-4 px-4 py-3 lg:px-6">
        <div className="flex min-w-0 items-center gap-6 sm:gap-8">
          <Link
            href="/experiments"
            className="flex shrink-0 items-center gap-2 font-semibold tracking-tight"
          >
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src="/logo.png"
              alt=""
              width={28}
              height={28}
              className="size-7"
            />
            <span>Dagobert</span>
          </Link>
          <nav className="flex flex-wrap gap-3 text-sm sm:gap-4">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={cn(
                  "transition-colors",
                  pathname === link.href || pathname.startsWith(link.href)
                    ? "text-foreground font-medium"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
        <Popover open={toolsOpen} onOpenChange={setToolsOpen}>
          <PopoverTrigger
            aria-label="Tools"
            title="Tools"
            className="inline-flex size-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground aria-expanded:bg-muted aria-expanded:text-foreground"
          >
            <Wrench className="size-4" aria-hidden />
          </PopoverTrigger>
          <PopoverContent align="end" sideOffset={6} className="w-44 gap-0.5 p-1">
            <Link
              href="/connectors"
              className="flex w-full rounded-md px-2.5 py-1.5 text-sm transition-colors hover:bg-muted"
              onClick={() => setToolsOpen(false)}
            >
              Connectors
            </Link>
          </PopoverContent>
        </Popover>
      </div>
    </header>
  );
}
