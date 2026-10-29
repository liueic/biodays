import { expect, test } from "bun:test";
import { normalizeSubmission } from "@/lib/backend/feedback";
import { feedbackArticle } from "@/lib/feedback-article";

test("feedback resolves stories and every lab section to their public article metadata", () => {
  for (const [id, path] of [
    ["Q1-01", "/story/Q1-01"],
    ["METH-03", "/lab/methods/METH-03"],
    ["PROT-01", "/lab/protocols/PROT-01"],
    ["TIP-01", "/lab/tips/TIP-01"],
  ]) {
    const article = feedbackArticle(id);
    expect(article?.id).toBe(id);
    expect(article?.path).toBe(path);
    expect(article?.title).not.toBeEmpty();
    expect(article?.summary).not.toBeEmpty();
    expect(
      normalizeSubmission({
        request_id: crypto.randomUUID(),
        kind: "issue",
        title: "文章问题",
        content: "请帮忙核对这篇文章的原理和引用。",
        story_id: id,
        subscribe_updates: false,
      }).story_id,
    ).toBe(id);
  }
  expect(feedbackArticle("")).toBeUndefined();
  expect(feedbackArticle("missing")).toBeUndefined();
});
