import { unstable_cache } from "next/cache";
import { cookies } from "next/headers";
import { cache } from "react";

import { getAllStories, getMeta, getStories, getTags, getToday } from "@/lib/api";

// Reuse within one request, including generateMetadata. Today's date is always read live.
export const getRequestToday = cache(async () => {
  const history = (await cookies()).get("biodays_read_v1");
  // Forward only this reader preference, never other site cookies. The result
  // stays within this render pass and is not added to a shared cache.
  return getToday(undefined, history ? `${history.name}=${history.value}` : undefined);
});

export const getCachedArchive = unstable_cache(
  async () => {
    const [initial, tags, meta] = await Promise.all([
      getStories({ page: 1, pageSize: 12 }),
      getTags(),
      getMeta(),
    ]);
    return { initial, tags: tags.data, meta: meta.data };
  },
  ["archive-summary-v1"],
  { revalidate: 300 },
);

export const getCachedRecent = unstable_cache(
  async () => {
    const first = await getStories({ page: 1, pageSize: 5 });
    const lastPage = Math.ceil(first.meta.total / 5);
    const last = lastPage > 1 ? await getStories({ page: lastPage, pageSize: 5 }) : first;
    return [...last.data].reverse();
  },
  ["recent-summary-v1"],
  { revalidate: 300 },
);

export const getCachedSitemapStories = unstable_cache(getAllStories, ["sitemap-summary-v1"], {
  revalidate: 300,
});
