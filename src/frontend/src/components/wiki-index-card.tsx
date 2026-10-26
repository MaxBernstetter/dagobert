
import type { ReactNode } from "react";
import Link from "@/components/link";
import { usePathname } from "@/lib/navigation";
import { cn } from "@/lib/utils";

export function WikiIndexCard({
  href,
  title,
  slugLine,
  metaLine,
  badges,
}: {
  href: string;
  title: string;
  slugLine: string;
  metaLine?: string;
  badges?: ReactNode;
}) {
  const pathname = usePathname();
  const active = pathname === href;

  return (
    <Link
      href={href}
      className={cn(
        "flex h-full min-h-[5.5rem] flex-col rounded-xl border border-border/70 bg-muted/30 px-3 py-3 shadow-sm transition-colors hover:bg-muted/50 dark:bg-muted/15",
        active && "border-primary bg-muted/50 ring-1 ring-primary/40"
      )}
    >
      <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
        <span className="text-sm font-medium leading-snug">{title}</span>
        {badges}
      </div>
      <span className="mt-1 truncate font-mono text-xs text-muted-foreground">{slugLine}</span>
      {metaLine ? (
        <span className="mt-1 line-clamp-2 text-xs leading-snug text-muted-foreground">{metaLine}</span>
      ) : null}
    </Link>
  );
}
