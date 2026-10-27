import { unstable_cache } from "next/cache";
import { cookies } from "next/headers";
import { cache } from "react";

import { datasetMeta, listStories, stories, storyByID, tagCounts } from "@/lib/content";
import { decodeHistory, todayStory } from "@/lib/reading";
export const getStory = async (id: string) => storyByID(id);
export const getMeta = async () => ({ data: datasetMeta() });
const getStories = async (query = {}) => listStories(query);
const getTags = async () => ({ data: tagCounts() });
const getAllStories = async () =>
  stories.map(({ content: _c, source: _s, geek_quote: _q, ...s }) => s);

// Reuse within one request, including generateMetadata. Today's date is always read live.
export const getRequestToday = cache(async () => {
  const history = (await cookies()).get("biodays_read_v1");
  // Forward only this reader preference, never other site cookies. The result
  // stays within this render pass and is not added to a shared cache.
  return todayStory(stories, "+08:00", "random", decodeHistory(history?.value));
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
