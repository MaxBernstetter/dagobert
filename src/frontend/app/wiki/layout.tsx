import { WikiIndex } from "@/components/wiki-index";

export default function WikiLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <header className="mb-4 shrink-0">
        <h1 className="text-2xl font-semibold tracking-tight">Wiki</h1>
        <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
          Pick an entry from the grid; its documentation opens in the panel on the right.
        </p>
      </header>
      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden lg:flex-row lg:gap-5">
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto lg:pr-1">
          <WikiIndex />
        </div>
        <aside
          className="flex max-h-[45vh] min-h-0 shrink-0 flex-col overflow-y-auto border-t border-border/60 pt-4 lg:max-h-none lg:w-[38%] lg:max-w-lg lg:border-l lg:border-t-0 lg:pl-4 lg:pt-0"
          aria-label="Wiki detail"
        >
          {children}
        </aside>
      </div>
    </div>
  );
}
