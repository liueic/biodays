import type { Metadata, Viewport } from "next";

import { AppShell } from "@/components/app-shell";
import { SITE_NAME, SITE_URL } from "@/lib/site";

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

// Private layout: no analytics or session-recording scripts here. Anchors
// into the (site) group stay plain links so the recorder, when it was ever
// loaded there, keeps being unloaded on the way in.
export default function PrivateLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
