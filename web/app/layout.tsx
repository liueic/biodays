import type { Metadata, Viewport } from "next";
import localFont from "next/font/local";
import Script from "next/script";

import { Providers } from "@/components/providers";
import { DesktopTaskbar } from "@/components/win/taskbar";
import { SITE_DESCRIPTION, SITE_NAME, SITE_TAGLINE, SITE_URL } from "@/lib/site";

import "./globals.css";
import "./react95-global.css";
import "@react95/core/themes/win95.css";

const pixelUI = localFont({
  src: "./fonts/pixel-ui.woff2",
  variable: "--font-pixel-ui",
  display: "swap",
  adjustFontFallback: false,
});

export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  title: {
    default: `${SITE_NAME} · ${SITE_TAGLINE}`,
    template: `%s · ${SITE_NAME}`,
  },
  description: SITE_DESCRIPTION,
  applicationName: SITE_NAME,
  keywords: ["实验室史", "科学史", "生物学", "每日故事", "biodays", "实验室器物"],
  authors: [{ name: SITE_NAME }],
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    locale: "zh_CN",
    url: "/",
    siteName: SITE_NAME,
    title: `${SITE_NAME} · ${SITE_TAGLINE}`,
    description: SITE_DESCRIPTION,
  },
  twitter: {
    card: "summary",
    title: `${SITE_NAME} · ${SITE_TAGLINE}`,
    description: SITE_DESCRIPTION,
  },
  robots: { index: true, follow: true },
};

export const viewport: Viewport = {
  themeColor: "#008080",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className={pixelUI.variable} suppressHydrationWarning>
      <body className="flex min-h-svh flex-col">
        <Providers>
          <a href="#main" className="skip-link">
            跳到正文
          </a>
          <div
            id="main"
            className="relative z-[1] flex flex-1 flex-col px-2 pt-2 pb-12 sm:px-6 sm:pt-6"
          >
            {/* Spare vertical space is split 0.382 : 0.618 (golden ratio), so short pages sit
                slightly above center; tall pages leave no spare space and just scroll. */}
            <div aria-hidden className="grow-[382] basis-0" />
            <div>{children}</div>
            <div aria-hidden className="grow-[618] basis-0" />
          </div>
          <DesktopTaskbar />
        </Providers>
        {process.env.NODE_ENV === "production" && (
          <>
            <Script
              id="umami-analytics"
              src="https://ana.juniortree.com/script.js"
              data-website-id="c3b4bed2-5ad6-4265-9c23-f9e462a829c4"
              strategy="afterInteractive"
              defer
            />
            <Script
              id="umami-recorder"
              src="https://ana.juniortree.com/recorder.js"
              data-website-id="c3b4bed2-5ad6-4265-9c23-f9e462a829c4"
              strategy="lazyOnload"
              defer
            />
          </>
        )}
      </body>
    </html>
  );
}
