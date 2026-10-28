import localFont from "next/font/local";
import { OsThemeScript } from "@/components/os-theme-script";

import "./globals.css";
import "./react95-global.css";
import "@react95/core/themes/win95.css";
import "./mac-global.css";

const pixelUI = localFont({
  src: "./fonts/pixel-ui.woff2",
  variable: "--font-pixel-ui",
  display: "swap",
  adjustFontFallback: false,
});

const chicago = localFont({
  src: "./fonts/chicago.woff2",
  variable: "--font-chicago",
  display: "swap",
  adjustFontFallback: false,
  preload: false,
});

// Minimal root layout: only the document shell shared by both route groups.
// Keeping it free of app chrome (taskbar, analytics) lets (site) and
// (private) stay behaviourally separate while app/not-found.tsx and
// app/global-error.tsx finally have a layout to render in — with two
// root-group layouts only, Next streamed unmatched 404s as an empty
// client-side-only shell.
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="zh-CN"
      className={`${pixelUI.variable} ${chicago.variable}`}
      suppressHydrationWarning
    >
      <head>
        <OsThemeScript />
      </head>
      <body className="flex min-h-svh flex-col">{children}</body>
    </html>
  );
}
