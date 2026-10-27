import { deflateSync, inflateSync } from "node:zlib";
import { ApiError, type Story } from "@/lib/api";

export const HISTORY_COOKIE = "biodays_read_v1";
export function decodeHistory(value = ""): string[] {
  if (value.length > 3500 || !/^[\w-]+$/.test(value)) return [];
  try {
    const ids: unknown = JSON.parse(
      inflateSync(Buffer.from(value, "base64url"), { maxOutputLength: 32768 }).toString(),
    );
    return Array.isArray(ids) &&
      ids.length <= 2048 &&
      ids.every(
        (id) =>
          typeof id === "string" &&
          Buffer.byteLength(id) > 0 &&
          Buffer.byteLength(id) <= 16 &&
          id.trim() === id,
      )
      ? ids
      : [];
  } catch {
    return [];
  }
}
export function encodeHistory(ids: string[], id: string) {
  let list = [...ids.filter((existing) => existing !== id), id].slice(-2048);
  for (;;) {
    const raw = JSON.stringify(list);
    const value = deflateSync(raw).toString("base64url");
    if (value.length <= 3500 && Buffer.byteLength(raw) <= 32768) return value;
    list = list.slice(Math.floor(list.length / 4) + 1);
  }
}
export function offsetMinutes(value: string) {
  const trimmed = value.trim();
  if (!trimmed || /^(Z|UTC)$/i.test(trimmed)) return 0;
  const match = /^([+-])(\d{2}):(\d{2})$/.exec(trimmed);
  if (
    !match ||
    Number(match[2]) > 14 ||
    Number(match[3]) > 59 ||
    (Number(match[2]) === 14 && match[3] !== "00")
  )
    throw new ApiError(400, "INVALID_PARAM", "invalid timezone offset");
  return (Number(match[2]) * 60 + Number(match[3])) * (match[1] === "-" ? -1 : 1);
}
function pick(items: Story[], rng: () => number) {
  if (!items.length) throw new ApiError(404, "STORY_NOT_FOUND", "no stories available");
  return items[Math.floor(rng() * items.length)];
}
export function randomUnread(
  stories: Story[],
  exclude: string[] = [],
  seen: string[] = [],
  rng = Math.random,
) {
  let candidates = stories.filter((s) => !exclude.includes(s.id));
  // Existing behavior permits returning the only story in a one-story dataset.
  if (!candidates.length) candidates = stories;
  const unread = candidates.filter((s) => !seen.includes(s.id));
  return { story: pick(unread.length ? unread : candidates, rng), reset: !unread.length };
}
export function todayStory(
  stories: Story[],
  tz = "+08:00",
  fallback = "random",
  seen: string[] = [],
  now = new Date(),
  rng = Math.random,
) {
  if (!["random", "none"].includes(fallback))
    throw new ApiError(400, "INVALID_PARAM", "fallback must be random or none");
  const local = new Date(now.getTime() + offsetMinutes(tz) * 60000);
  const matches = stories.filter(
    (s) => s.month === local.getUTCMonth() + 1 && s.day === local.getUTCDate(),
  );
  if (matches.length) {
    const unread = matches.filter((s) => !seen.includes(s.id));
    return { data: pick(unread.length ? unread : matches, rng), meta: { is_fallback: false } };
  }
  if (fallback === "none") throw new ApiError(404, "STORY_NOT_FOUND", "no story for today");
  return { data: randomUnread(stories, [], seen, rng).story, meta: { is_fallback: true } };
}
