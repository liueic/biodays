import { describe, expect, test } from "bun:test";
import { deflateSync } from "node:zlib";
import { datasetMeta, listStories, stories, storyByID } from "@/lib/content";
import {
  decodeHistory,
  encodeHistory,
  offsetMinutes,
  randomUnread,
  todayStory,
} from "@/lib/reading";

describe("reading compatibility", () => {
  test("complete dataset and stable lookup/pagination", () => {
    expect(stories.length).toBeGreaterThanOrEqual(408);
    expect(new Set(stories.map((s) => s.id)).size).toBe(stories.length);
    const all = Array.from({ length: Math.ceil(stories.length / 100) }, (_, i) => i + 1).flatMap(
      (page) => listStories({ page, pageSize: 100 }).data,
    );
    expect(all.map((s) => s.id)).toEqual(stories.map((s) => s.id));
    expect(listStories({ q: "Eppendorf" }).data.length).toBeGreaterThan(0);
    expect(
      listStories({ month: 2, day: 29 }).data.every((s) => s.month === 2 && s.day === 29),
    ).toBe(true);
    expect(storyByID("Q1-01").source.url).toBe("https://www.eppendorf.com/");
    expect(() => storyByID("../bad")).toThrow();
    expect(datasetMeta().total).toBe(stories.length);
  });
  test("timezone boundaries, invalid offsets and fallback", () => {
    const sample = [
      { ...stories[0], id: "a", month: 10, day: 16 },
      { ...stories[0], id: "b", month: 10, day: 16 },
      { ...stories[0], id: "c", month: 2, day: 29 },
    ];
    expect(
      todayStory(sample, "+08:00", "none", ["a"], new Date("2026-10-15T20:00:00Z")).data.id,
    ).toBe("b");
    expect(() => todayStory(sample, "UTC", "none", [], new Date("2026-10-15T20:00:00Z"))).toThrow();
    expect(
      todayStory(sample, "UTC", "random", [], new Date("2026-10-15T20:00:00Z")).meta.is_fallback,
    ).toBe(true);
    expect(todayStory(sample, "UTC", "none", [], new Date("2028-02-29T00:00:00Z")).data.id).toBe(
      "c",
    );
    expect(
      todayStory(sample, "UTC", "none", ["a", "b"], new Date("2026-10-16T00:00:00Z")).meta
        .is_fallback,
    ).toBe(false);
    for (const invalid of ["8", "+15:00", "+08:60", "08:00", "+8:00", "Asia/Shanghai", "+14:01"])
      expect(() => offsetMinutes(invalid)).toThrow();
    expect(offsetMinutes("-05:30")).toBe(-330);
    expect(offsetMinutes("+14:00")).toBe(840);
  });
  test("unread round resets preserve explicit exclusions", () => {
    const sample = stories.slice(0, 3);
    const seen = sample.map((s) => s.id);
    const draw = randomUnread(sample, [sample[0].id], seen, () => 0);
    expect(draw.reset).toBe(true);
    expect(draw.story.id).not.toBe(sample[0].id);
    expect(randomUnread(sample, [], seen.slice(0, 2), () => 0).story.id).toBe(sample[2].id);
    expect(randomUnread(sample.slice(0, 1), [sample[0].id], [], () => 0).story.id).toBe(
      sample[0].id,
    );
  });
  test("Go-compatible zlib cookie, corruption and bounds", () => {
    const ids = stories.map((s) => s.id);
    const encoded = encodeHistory(ids, ids[0]);
    expect(encoded.length).toBeLessThanOrEqual(3500);
    expect(decodeHistory(encoded)).toHaveLength(ids.length);
    expect(
      decodeHistory(deflateSync(JSON.stringify(["Q1-01", "BIO-C1-01"])).toString("base64url")),
    ).toEqual(["Q1-01", "BIO-C1-01"]);
    for (const value of [
      "",
      "broken",
      "a".repeat(3501),
      deflateSync(JSON.stringify([" invalid "])).toString("base64url"),
      deflateSync("a".repeat(40000)).toString("base64url"),
    ])
      expect(decodeHistory(value)).toEqual([]);
  });
});
