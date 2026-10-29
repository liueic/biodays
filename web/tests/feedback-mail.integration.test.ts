import { expect, test } from "bun:test";
import { db } from "@/lib/backend/db";
import { getFeedback, submit, subscriptionAction, updateFeedback } from "@/lib/backend/feedback";
import { defaults } from "@/lib/backend/settings";
import { feedbackArticle } from "@/lib/feedback-article";

const local = /^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "");
test.skipIf(!local)(
  "admin and confirmed subscriber emails include trusted article context without exposing it in confirmation mail",
  async () => {
    const previous = {
      BIODAYS_NOTIFY_TO: process.env.BIODAYS_NOTIFY_TO,
      RESEND_FROM: process.env.RESEND_FROM,
      RESEND_API_KEY: process.env.RESEND_API_KEY,
    };
    process.env.BIODAYS_NOTIFY_TO = "admin@biodays.test";
    process.env.RESEND_FROM = "sender@biodays.test";
    process.env.RESEND_API_KEY = "re_local_test";
    const config = defaults();
    config.resend.enabled = true;
    const ids: string[] = [];
    const origin = new URL(process.env.BIODAYS_SITE_URL ?? "http://localhost:13000").origin;
    const request = new Request(`${origin}/api/v1/feedback`, {
      headers: { "x-forwarded-for": `fd00::${crypto.randomUUID().slice(0, 4)}` },
    });
    try {
      for (const articleId of ["Q1-01", "METH-03", ""]) {
        const result = await submit(
          {
            request_id: crypto.randomUUID(),
            kind: "issue",
            title: "请核对文章中的科学依据",
            content: "关于本文的描述，我有一些需要核对的地方，请参考原始研究。",
            source_url: "https://example.com/unrelated-resource",
            story_id: articleId,
            email: `article-${crypto.randomUUID()}@biodays.test`,
            subscribe_updates: true,
          },
          request,
          config,
        );
        const id = result.data.id as string;
        ids.push(id);
        const queued = await db()
          .from("biodays_outbox")
          .select("type,payload")
          .eq("feedback_id", id);
        expect(queued.error).toBeNull();
        const adminMail = queued.data?.find((mail) => mail.type === "admin_new_feedback");
        const confirmation = queued.data?.find((mail) => mail.type === "subscription_confirm");
        expect(adminMail).toBeDefined();
        expect(confirmation).toBeDefined();
        const token = /#token=([\w-]+)/.exec(confirmation?.payload.text ?? "")?.[1] ?? "";
        await subscriptionAction({ token }, "confirm", request, config);
        const updated = await updateFeedback(
          id,
          {
            version: (await getFeedback(id)).version,
            status: "accepted",
            public_reply: "谢谢，已开始核对。",
            internal_note: "PRIVATE_INTERNAL_NOTE",
            notify_subscriber: true,
          },
          "local-article-mail-test",
        );
        expect(updated.meta.notified).toBe(true);
        const statusMail = await db()
          .from("biodays_outbox")
          .select("payload")
          .eq("feedback_id", id)
          .eq("type", "user_status_update")
          .single();
        expect(statusMail.error).toBeNull();
        const article = feedbackArticle(articleId);
        for (const payload of [adminMail?.payload, statusMail.data?.payload]) {
          expect(payload).toBeDefined();
          if (article) {
            expect(payload.text).toContain(`关联文章：${article.title}（${article.id}）`);
            expect(payload.text).toContain(`摘要：${article.summary}`);
            expect(payload.text).toContain(`阅读全文：${origin}${article.path}`);
            expect(payload.html).toContain(`href="${origin}${article.path}"`);
          } else {
            expect(payload.text).not.toContain("关联文章：");
          }
        }
        if (article) {
          expect(confirmation?.payload.text).not.toContain(article.title);
          expect(updated.data.related_article).toEqual(article);
        }
        expect(statusMail.data?.payload.text).not.toContain("PRIVATE_INTERNAL_NOTE");
      }
    } finally {
      if (ids.length) await db().from("biodays_feedback").delete().in("id", ids);
      for (const [key, value] of Object.entries(previous)) {
        if (value === undefined) delete process.env[key];
        else process.env[key] = value;
      }
    }
  },
);
