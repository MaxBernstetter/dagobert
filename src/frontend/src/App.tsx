import { ExperimentsPanel } from "@/components/experiments-panel";
import { IngestionProvidersPanel } from "@/components/ingestion-providers-panel";
import { BreadcrumbProvider, PageBreadcrumbs, RouteBreadcrumbs } from "@/components/page-breadcrumbs";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { TradingConnectorsPanel } from "@/components/trading-connectors-panel";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { WikiHome } from "./WikiPages";

export function App() {
  return (
    <BrowserRouter>
      <BreadcrumbProvider>
      <div className="flex h-dvh flex-col overflow-hidden bg-background text-foreground antialiased">
        <SiteHeader />
        <RouteBreadcrumbs />
        <PageBreadcrumbs />
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden px-4 py-4 lg:px-6">
          <Routes>
            <Route path="/" element={<ExperimentsPanel className="min-h-0 flex-1" />} />
            <Route path="/experiments" element={<ExperimentsPanel className="min-h-0 flex-1" />} />
            <Route path="/wiki/*" element={<WikiHome />} />
            <Route path="/connectors" element={<ConnectorsPage />} />
            <Route path="/ingestion" element={<ConnectorsPage />} />
          </Routes>
        </main>
        <SiteFooter />
      </div>
      </BreadcrumbProvider>
    </BrowserRouter>
  );
}

function ConnectorsPage() {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8 overflow-auto">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Connectors</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Market-data ingestion and trading venue credentials.
        </p>
      </div>
      <section className="flex min-h-0 flex-col gap-3">
        <h2 className="text-base font-semibold tracking-tight">Ingestion</h2>
        <IngestionProvidersPanel />
      </section>
      <section className="flex flex-col gap-3 border-t pt-8">
        <h2 className="text-base font-semibold tracking-tight">Trading</h2>
        <p className="text-sm text-muted-foreground">
          Execution venues. Secrets are read from .secrets and shown masked only.
        </p>
        <TradingConnectorsPanel />
      </section>
    </div>
  );
}
