"use client";

import { ChevronRight } from "lucide-react";
import { usePathname } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type PageCrumb = {
  label: string;
  onClick?: () => void;
};

type BreadcrumbContextValue = {
  crumbs: PageCrumb[];
  setCrumbs: (crumbs: PageCrumb[]) => void;
};

const BreadcrumbContext = createContext<BreadcrumbContextValue | null>(null);

function sameCrumbs(a: PageCrumb[], b: PageCrumb[]): boolean {
  if (a.length !== b.length) return false;
  return a.every(
    (crumb, index) =>
      crumb.label === b[index].label && Boolean(crumb.onClick) === Boolean(b[index].onClick),
  );
}

export function BreadcrumbProvider({ children }: { children: ReactNode }) {
  const [crumbs, setCrumbState] = useState<PageCrumb[]>([]);
  const setCrumbs = useCallback((next: PageCrumb[]) => {
    setCrumbState((prev) => (sameCrumbs(prev, next) ? prev : next));
  }, []);
  const value = useMemo(() => ({ crumbs, setCrumbs }), [crumbs, setCrumbs]);
  return <BreadcrumbContext.Provider value={value}>{children}</BreadcrumbContext.Provider>;
}

export function usePageBreadcrumbs(): BreadcrumbContextValue {
  const value = useContext(BreadcrumbContext);
  if (!value) throw new Error("usePageBreadcrumbs must be used inside BreadcrumbProvider");
  return value;
}

/** Default trail for routes that do not publish their own. */
export function RouteBreadcrumbs() {
  const pathname = usePathname();
  const { setCrumbs } = usePageBreadcrumbs();
  useLayoutEffect(() => {
    if (pathname.startsWith("/wiki")) setCrumbs([{ label: "Wiki" }]);
    else if (pathname.startsWith("/connectors") || pathname.startsWith("/ingestion")) {
      setCrumbs([{ label: "Connectors" }]);
    }
  }, [pathname, setCrumbs]);
  return null;
}

export function PageBreadcrumbs() {
  const { crumbs } = usePageBreadcrumbs();
  if (crumbs.length === 0) return null;
  return (
    <nav aria-label="Breadcrumb" className="shrink-0 border-b bg-background">
      <ol className="flex min-w-0 items-center gap-1 px-4 py-2 text-sm lg:px-6">
        {crumbs.map((crumb, index) => {
          const current = index === crumbs.length - 1;
          return (
            <li key={`${crumb.label}-${index}`} className="flex min-w-0 items-center gap-1">
              {index > 0 && <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />}
              {current || !crumb.onClick ? (
                <span className={current ? "truncate font-medium" : "truncate text-muted-foreground"} aria-current={current ? "page" : undefined}>
                  {crumb.label}
                </span>
              ) : (
                <button
                  type="button"
                  className="truncate text-muted-foreground transition-colors hover:text-foreground"
                  onClick={crumb.onClick}
                >
                  {crumb.label}
                </button>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
