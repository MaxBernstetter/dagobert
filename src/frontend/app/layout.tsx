import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { SiteHeader } from "@/components/site-header";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Trading Copilot",
  description: "Overview of balances, decisions, and fills",
};

const themeInit = `(function(){try{var t=localStorage.getItem('theme');var d=t==='dark'||(t!=='light'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);}catch(e){}})();`;

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInit }} />
      </head>
      <body
        className={`${geistSans.variable} ${geistMono.variable} flex h-dvh flex-col overflow-hidden antialiased bg-background text-foreground`}
      >
        <SiteHeader />
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden px-4 py-4 lg:px-6">
          {children}
        </main>
        <footer className="shrink-0 border-t bg-background">
          <div className="flex w-full items-center justify-between gap-4 px-4 py-2 text-xs text-muted-foreground lg:px-6">
            <span>Local stack · data under ./data</span>
            <span>Trading Copilot</span>
          </div>
        </footer>
      </body>
    </html>
  );
}
