import type { Metadata } from "next";

import { HomeDesktop } from "@/components/home-desktop";
import { getStories, getToday, type Story } from "@/lib/api";
import { getTodayStamp } from "@/lib/date";
import { SITE_DESCRIPTION, SITE_NAME, SITE_TAGLINE } from "@/lib/site";

export const revalidate = 300;

export async function generateMetadata(): Promise<Metadata> {
  try {
    const today = await getToday();
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

const RECENT_PAGE_SIZE = 5;

/** Latest stories by archive order (the list sorts ascending by month/day, so take the last page). */
async function getRecentStories(): Promise<Story[]> {
  const first = await getStories({ page: 1, pageSize: RECENT_PAGE_SIZE });
  const totalPages = Math.ceil(first.meta.total / RECENT_PAGE_SIZE);
  const last =
    totalPages > 1 ? await getStories({ page: totalPages, pageSize: RECENT_PAGE_SIZE }) : first;
  return [...last.data].reverse();
}

export default async function HomePage() {
  const stamp = getTodayStamp();
  const [today, recent] = await Promise.all([
    getToday().catch(() => null),
    getRecentStories().catch(() => [] as Story[]),
  ]);

  return <HomeDesktop today={today} recent={recent} stamp={stamp} />;
}
