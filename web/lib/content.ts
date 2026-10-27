// Server-only content repository. The generated index is bundled by Next/Vercel;
// no runtime filesystem access or database is needed to read a story.
import index from "@/content/index.json";
import { ApiError, type Story, type StoryQuery } from "@/lib/api";

export const stories: Story[] = index.stories;
export function storyByID(id: string): Story {
  const story = stories.find((story) => story.id === id);
  if (!story) throw new ApiError(404, "STORY_NOT_FOUND", "story not found");
  return story;
}
export function summary(story: Story) {
  const { content: _content, source: _source, geek_quote: _quote, ...rest } = story;
  return rest;
}
export function listStories(query: StoryQuery = {}, full = false) {
  const page = query.page ?? 1;
  const pageSize = query.pageSize ?? 20;
  const q = query.q?.trim().toLocaleLowerCase();
  const filtered = stories.filter(
    (s) =>
      (!query.month || s.month === query.month) &&
      (!query.day || s.day === query.day) &&
      (!query.tag || s.tags.includes(query.tag)) &&
      (!q ||
        [s.title, s.summary, s.lab_item].some((value) => value.toLocaleLowerCase().includes(q))),
  );
  return {
    data: filtered
      .slice((page - 1) * pageSize, page * pageSize)
      .map((s) => (full ? s : summary(s))),
    meta: { total: filtered.length, page, page_size: pageSize },
  };
}
export function tagCounts() {
  const counts = new Map<string, number>();
  for (const s of stories)
    for (const tag of new Set(s.tags)) counts.set(tag, (counts.get(tag) ?? 0) + 1);
  return [...counts]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}
export function datasetMeta() {
  const month_counts: Record<number, number> = {};
  for (const s of stories) month_counts[s.month] = (month_counts[s.month] ?? 0) + 1;
  const months = Object.keys(month_counts)
    .map(Number)
    .sort((a, b) => a - b);
  const ranges: string[] = [];
  for (let i = 0; i < months.length; i++) {
    const start = months[i];
    while (months[i + 1] === months[i] + 1) i++;
    ranges.push(start === months[i] ? `${start}月` : `${start}-${months[i]}月`);
  }
  return {
    total: stories.length,
    quarters: [...new Set(months.map((m) => `Q${Math.ceil(m / 3)}`))],
    coverage: ranges.join(" / "),
    month_counts,
    dataset_version: index.version,
  };
}
