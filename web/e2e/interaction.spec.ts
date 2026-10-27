import { createHmac } from "node:crypto";
import { expect, test } from "@playwright/test";
import { createClient } from "@supabase/supabase-js";

test.skip(
  !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "") ||
    !process.env.SUPABASE_SECRET_KEY,
  "Requires an isolated local Supabase project",
);
test.beforeEach(async () => {
  const client = createClient(
    process.env.SUPABASE_URL ?? "",
    process.env.SUPABASE_SECRET_KEY ?? "",
  );
  await client.from("biodays_abuse").delete().neq("key_hash", "");
});
test("feedback, double opt-in, authenticated review, settings CAS and unsubscribe", async ({
  request,
  page,
}) => {
  const db = createClient(process.env.SUPABASE_URL ?? "", process.env.SUPABASE_SECRET_KEY ?? "", {
    auth: { persistSession: false },
  });
  const headers = { Origin: "http://localhost:13000" };
  const input = {
    request_id: crypto.randomUUID(),
    kind: "issue",
    title: `回归测试 ${crypto.randomUUID().slice(0, 8)}`,
    content: "这是一条用于验证迁移后完整反馈流程的测试内容。",
    email: `reader-${crypto.randomUUID().slice(0, 8)}@biodays.test`,
    subscribe_updates: true,
    story_id: "Q1-01",
  };
  const response = await request.post("/api/v1/feedback", { headers, data: input });
  expect(response.status()).toBe(201);
  const receipt = (await response.json()).data,
    id = receipt.id;
  const repeat = await request.post("/api/v1/feedback", { headers, data: input });
  expect(repeat.status()).toBe(200);
  expect((await repeat.json()).data.id).toBe(id);
  expect(
    (
      await request.post("/api/v1/feedback", { headers, data: { ...input, title: "不同内容" } })
    ).status(),
  ).toBe(409);
  const confirmation = await db
    .from("biodays_outbox")
    .select("payload")
    .eq("feedback_id", id)
    .eq("type", "subscription_confirm")
    .single();
  expect(confirmation.error).toBeNull();
  expect(confirmation.data?.payload.text).not.toContain(input.title);
  const token = /#token=([\w-]+)/.exec(confirmation.data?.payload.text ?? "")?.[1] ?? "";
  await page.goto(`/feedback/confirm#token=${token}`);
  expect(
    (await db.from("biodays_feedback").select("subscription_status").eq("id", id).single()).data
      ?.subscription_status,
  ).toBe("pending_confirmation");
  expect(
    (
      await request.post("/api/v1/feedback/subscriptions/confirm", { headers, data: { token } })
    ).status(),
  ).toBe(200);
  expect((await request.get(`/api/v1/admin/feedback/${id}`)).status()).toBe(401);
  await page.goto("/admin/login");
  await page.getByLabel("邮箱", { exact: true }).fill(process.env.BIODAYS_ADMIN_EMAIL ?? "");
  await page.getByLabel("密码", { exact: true }).fill(process.env.BIODAYS_ADMIN_PASSWORD ?? "");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page).toHaveURL(/\/admin\/feedback$/);
  const login = await request.post("/api/v1/admin/login", {
    headers,
    data: {
      username: process.env.BIODAYS_ADMIN_EMAIL,
      password: process.env.BIODAYS_ADMIN_PASSWORD,
    },
  });
  expect(login.status()).toBe(200);
  const csrf = (await login.json()).data.csrf_token,
    adminHeaders = { ...headers, "X-CSRF-Token": csrf };
  const detail = (await (await request.get(`/api/v1/admin/feedback/${id}`)).json()).data;
  const updated = await request.patch(`/api/v1/admin/feedback/${id}`, {
    headers: adminHeaders,
    data: {
      version: detail.version,
      status: "accepted",
      internal_note: "PRIVATE_INTERNAL_NOTE",
      public_reply: "谢谢，我们已经采纳。",
      notify_subscriber: true,
    },
  });
  expect(updated.status()).toBe(200);
  const notification = await db
    .from("biodays_outbox")
    .select("id,payload,status")
    .eq("feedback_id", id)
    .eq("type", "user_status_update")
    .single();
  expect(notification.error).toBeNull();
  expect(notification.data?.payload.text ?? "").not.toContain("PRIVATE_INTERNAL_NOTE");
  const unsubToken = /#token=([\w-]+)/.exec(notification.data?.payload.text ?? "")?.[1] ?? "";
  expect(
    (
      await request.post("/api/v1/feedback/subscriptions/unsubscribe", {
        headers,
        data: { token: unsubToken },
      })
    ).status(),
  ).toBe(200);
  expect(
    (
      await db
        .from("biodays_outbox")
        .select("status,payload")
        .eq("id", notification.data?.id ?? "")
        .single()
    ).data,
  ).toEqual({ status: "cancelled", payload: null });
  const initial = (await (await request.get("/api/v1/admin/settings")).json()).data;
  const saves = await Promise.all(
    [true, false].map((enabled) =>
      request.patch("/api/v1/admin/settings", {
        headers: adminHeaders,
        data: { base_revision: initial.revision, subscriptions_enabled: enabled },
      }),
    ),
  );
  expect(saves.map((r) => r.status()).sort()).toEqual([200, 409]);
  // Restore a deterministic default for the next browser project.
  const current = (await (await request.get("/api/v1/admin/settings")).json()).data;
  await request.patch("/api/v1/admin/settings", {
    headers: adminHeaders,
    data: { base_revision: current.revision, subscriptions_enabled: true },
  });
  expect(
    (await request.post("/api/v1/admin/stories", { headers: adminHeaders, data: {} })).status(),
  ).toBe(410);
  expect((await request.post("/api/v1/admin/logout", { headers: adminHeaders })).status()).toBe(
    200,
  );
  expect((await request.get("/api/v1/admin/me")).status()).toBe(401);
  const anon = createClient(
    process.env.SUPABASE_URL ?? "",
    process.env.SUPABASE_PUBLISHABLE_KEY ?? "",
    {
      auth: { persistSession: false },
    },
  );
  expect((await anon.from("biodays_feedback").select("*")).error).not.toBeNull();
  expect(
    (await anon.auth.signUp({ email: "signup@biodays.test", password: "PublicSignupMustFail123!" }))
      .error,
  ).not.toBeNull();
  await db.from("biodays_feedback").delete().eq("id", id);
});

test("Resend SDK verifies signed webhooks and rejects replays without duplicates", async ({
  request,
}) => {
  test.skip(!process.env.RESEND_WEBHOOK_SECRET, "Requires local webhook signing key");
  const secret = process.env.RESEND_WEBHOOK_SECRET ?? "",
    id = `msg_${crypto.randomUUID()}`,
    timestamp = String(Math.floor(Date.now() / 1000));
  const payload = JSON.stringify({
    type: "email.bounced",
    created_at: new Date().toISOString(),
    data: {
      email_id: `test-${id}`,
      to: ["bounce@biodays.test"],
      from: "test@biodays.test",
      subject: "test",
    },
  });
  const signature = createHmac("sha256", Buffer.from(secret.replace(/^whsec_/, ""), "base64"))
    .update(`${id}.${timestamp}.${payload}`)
    .digest("base64");
  const headers = {
    "Content-Type": "application/json",
    "svix-id": id,
    "svix-timestamp": timestamp,
    "svix-signature": `v1,${signature}`,
  };
  expect((await request.post("/api/webhooks/resend", { headers, data: payload })).status()).toBe(
    200,
  );
  expect((await request.post("/api/webhooks/resend", { headers, data: payload })).status()).toBe(
    200,
  );
  expect(
    (
      await request.post("/api/webhooks/resend", {
        headers: { ...headers, "svix-signature": "v1,invalid" },
        data: payload,
      })
    ).status(),
  ).toBe(400);
  const db = createClient(process.env.SUPABASE_URL ?? "", process.env.SUPABASE_SECRET_KEY ?? "");
  expect(
    (await db.from("biodays_webhooks").select("id", { count: "exact" }).eq("id", id)).count,
  ).toBe(1);
  await db.from("biodays_webhooks").delete().eq("id", id);
  await db.from("biodays_suppressions").delete().eq("email", "bounce@biodays.test");
});

test("Jev settings save privately, validate probabilities and survive a page reload", async ({
  page,
}) => {
  const db = createClient(process.env.SUPABASE_URL ?? "", process.env.SUPABASE_SECRET_KEY ?? "", {
    auth: { persistSession: false },
  });
  const original = await db.from("biodays_settings").select("document").eq("id", true).single();
  expect(original.error).toBeNull();
  try {
    await page.goto("/admin/login");
    await page.getByLabel("邮箱", { exact: true }).fill(process.env.BIODAYS_ADMIN_EMAIL ?? "");
    await page.getByLabel("密码", { exact: true }).fill(process.env.BIODAYS_ADMIN_PASSWORD ?? "");
    await page.getByRole("button", { name: "登录", exact: true }).click();
    await expect(page).toHaveURL(/\/admin\/feedback$/);
    await page.goto("/admin/settings");
    await expect(page.getByLabel("模型", { exact: true })).toHaveValue("typesafe-ai/jev-1.14.0");
    await page.getByLabel("AI Gateway API Key").fill("gateway-ui-test-key");
    await page.getByLabel("启用 AI 复核", { exact: true }).check();
    await page.getByRole("button", { name: "保存设置", exact: true }).click();
    await expect(page.getByRole("status")).toHaveText("设置已保存，下一次请求生效。");
    await expect(page.getByLabel("AI Gateway API Key")).toHaveValue("");
    const saved = await page.request.get("/api/v1/admin/settings");
    expect(saved.status()).toBe(200);
    const text = await saved.text();
    expect(text).not.toMatch(/gateway-ui-test-key|v1:/);
    expect(JSON.parse(text).data.ai_spam.api_key).toMatchObject({
      configured: true,
      encrypted: true,
      readable: true,
      source: "database",
    });
    await page.reload();
    await expect(page.getByLabel("启用 AI 复核", { exact: true })).toBeChecked();
    await expect(page.getByLabel("AI Gateway API Key")).toHaveValue("");
    await page.getByLabel("垃圾概率上限").fill("0.5");
    await page.getByRole("button", { name: "保存设置", exact: true }).click();
    await expect(page.getByRole("alert").filter({ hasText: "AI 参数无效。" })).toBeVisible();
    expect((await (await page.request.get("/api/v1/admin/settings")).json()).data.revision).toBe(
      JSON.parse(text).data.revision,
    );
  } finally {
    // This local test never submits feedback while the synthetic key is enabled.
    await db.from("biodays_settings").update({ document: original.data?.document }).eq("id", true);
  }
});
