import { expect, test } from "@playwright/test";
import { createClient } from "@supabase/supabase-js";
import { signInLocalIdentity } from "./helpers/admin";

test("lab article feedback keeps the article association through the form and admin inbox", async ({
  page,
}) => {
  test.skip(
    !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "") ||
      !process.env.SUPABASE_SECRET_KEY,
    "Requires an isolated local Supabase project",
  );
  let feedbackId = "";
  const client = createClient(
    process.env.SUPABASE_URL ?? "",
    process.env.SUPABASE_SECRET_KEY ?? "",
  );
  try {
    await page.goto("/lab/methods/METH-03");
    const articleTitle = (await page.getByRole("heading", { level: 1 }).textContent()) ?? "";
    await page.getByRole("link", { name: "提交反馈", exact: true }).click();
    await expect(page).toHaveURL(/\/feedback\?kind=issue&story_id=METH-03$/);
    await page.locator("#feedback-title").fill("请核对这篇技术解读的描述");
    await page
      .getByRole("textbox", { name: "正文", exact: true })
      .fill("关于本文的原理描述，我想补充一些信息，请帮忙核对原始论文。");
    await page.getByText("邮件选项（相关链接、通知与隐私）", { exact: true }).click();
    await expect(page.getByText("关联文章：METH-03", { exact: true })).toBeVisible();
    const saved = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/v1/feedback") && response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "发送(S)", exact: true }).click();
    const response = await saved;
    expect(response.status()).toBe(201);
    feedbackId = (await response.json()).data.id;
    await signInLocalIdentity(page);
    await page.goto(`/admin/feedback/${feedbackId}`);
    await expect(page.getByRole("link", { name: articleTitle, exact: true })).toHaveAttribute(
      "href",
      "/lab/methods/METH-03",
    );
  } finally {
    if (feedbackId) await client.from("biodays_feedback").delete().eq("id", feedbackId);
  }
});
