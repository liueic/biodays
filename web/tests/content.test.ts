import { expect, test } from "bun:test";
import { readFile } from "node:fs/promises";
import type { Story } from "@/lib/api";
import { loadContent } from "../scripts/content.mjs";

test("every original story survives Markdown conversion with all fields and exact body", async () => {
  const originals = JSON.parse(
    await readFile(new URL("./fixtures/legacy-stories.json", import.meta.url), "utf8"),
  );
  const { stories } = await loadContent();
  expect(stories.length).toBeGreaterThanOrEqual(originals.length);
  for (const story of originals)
    expect((stories as Story[]).find((s) => s.id === story.id)).toEqual(story);
});
