import { expect, test } from "bun:test";
import { getLabArticle, getLabArticles, LAB_SECTIONS, type LabArticle } from "@/lib/lab";
import { loadLabContent } from "../scripts/lab-content.mjs";

test("loadLabContent validates all lab articles with required metadata", async () => {
  const { articles, version } = await loadLabContent();
  const labList = articles as LabArticle[];
  expect(labList.length).toBeGreaterThanOrEqual(20);
  expect(version.startsWith("sha256:")).toBe(true);

  for (const article of labList) {
    expect(article.id).toBeString();
    expect(article.title.trim()).not.toBeEmpty();
    expect(article.summary.trim()).not.toBeEmpty();
    expect(article.content.trim()).not.toBeEmpty();
    expect(article.lab_item.trim()).not.toBeEmpty();
    expect(article.geek_quote.trim()).not.toBeEmpty();
    expect(article.topic.trim()).not.toBeEmpty();
    expect(Array.isArray(article.tags)).toBe(true);
    expect(article.tags.length).toBeGreaterThan(0);
    expect(article.source).toBeDefined();
    expect(article.source.type).toBeString();
    expect(article.source.citation.trim()).not.toBeEmpty();
    expect(article.source.url.startsWith("https://")).toBe(true);
    expect(["methods", "protocols", "tips"].includes(article.section)).toBe(true);
  }
});

test("lab helpers return populated collections across all sections", () => {
  for (const section of LAB_SECTIONS) {
    const sectionArticles = getLabArticles(section.id);
    expect(sectionArticles.length).toBeGreaterThan(0);
    for (const item of sectionArticles) {
      expect(item.section).toBe(section.id);
      expect(getLabArticle(item.id)).toEqual(item);
    }
  }

  const tip1 = getLabArticle("TIP-01");
  expect(tip1).toBeDefined();
  expect(tip1?.title).toContain("漂白水");
  expect(tip1?.source.citation).toContain("Aranda");

  const prot1 = getLabArticle("PROT-01");
  expect(prot1).toBeDefined();
  expect(prot1?.title).toContain("SLIC");
  expect(prot1?.source.citation).toContain("Elledge");
});
