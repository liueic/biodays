import type { Metadata } from "next";

import { HomeDesktop } from "@/components/home-desktop";
import { getTodayStamp } from "@/lib/date";
import { getDailyLabRecommendations } from "@/lib/lab-recommendations";
import { getCachedRecent, getRequestToday } from "@/lib/server-api";
import { SITE_DESCRIPTION, SITE_NAME, SITE_TAGLINE } from "@/lib/site";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  try {
    const today = await getRequestToday();
    return {
      title: today.data.title,
      description: today.data.summary,
      alternates: { canonical: "/" },
      openGraph: {
        type: "article",
        title: today.data.title,
        description: today.data.summary,
        url: "/",
        siteName: SITE_NAME,
      },
    };
  } catch {
    return {
      title: `${SITE_NAME} · ${SITE_TAGLINE}`,
      description: SITE_DESCRIPTION,
      alternates: { canonical: "/" },
    };
  }
}

export default async function HomePage() {
  const stamp = getTodayStamp();
  const [today, recent] = await Promise.all([
    getRequestToday().catch(() => null),
    getCachedRecent().catch(() => []),
  ]);

  return (
    <HomeDesktop
      today={today}
      recent={recent}
      stamp={stamp}
      labRecommendations={getDailyLabRecommendations(stamp)}
    />
  );
}
