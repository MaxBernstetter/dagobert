import type { Metadata } from "next";
import Link from "next/link";
import { Geist, Geist_Mono } from "next/font/google";
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

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased min-h-screen bg-background text-foreground`}
      >
        <header className="border-b">
          <div className="mx-auto flex max-w-6xl items-center gap-8 px-4 py-4">
            <span className="font-semibold tracking-tight">Trading Copilot</span>
            <nav className="flex gap-4 text-sm">
              <Link
                href="/"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                Portfolio
              </Link>
              <Link
                href="/orders"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                Orders
              </Link>
              <Link
                href="/data"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                Data
              </Link>
              <Link
                href="/processing"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                Processing
              </Link>
            </nav>
          </div>
        </header>
        <main className="mx-auto max-w-6xl px-4 py-8">{children}</main>
      </body>
    </html>
  );
}
