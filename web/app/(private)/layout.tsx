import type { Metadata, Viewport } from "next";
import localFont from "next/font/local";

import { AppShell } from "@/components/app-shell";
import { SITE_NAME, SITE_URL } from "@/lib/site";

import "../globals.css";
import "../react95-global.css";
import "@react95/core/themes/win95.css";

const pixelUI = localFont({
  src: "../fonts/pixel-ui.woff2",
  variable: "--font-pixel-ui",
  display: "swap",
  adjustFontFallback: false,
});

export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  title: {
    default: `灵感信箱 · ${SITE_NAME}`,
    template: `%s · ${SITE_NAME}`,
  },
  description: "biodays 匿名反馈、订阅确认与管理员收件箱。",
  robots: { index: false, follow: false, nocache: true },
  referrer: "no-referrer",
};

export const viewport: Viewport = {
  themeColor: "#008080",
};

// Private root layout: no analytics or session-recording scripts here. Moving
// between the reading layout and this one is a full document load, which also
// unloads any recorder started on a story page.
export default function PrivateLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className={pixelUI.variable} suppressHydrationWarning>
      <body className="flex min-h-svh flex-col">
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}
