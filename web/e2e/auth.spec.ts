import { expect, test } from "@playwright/test";
import { createClient } from "@supabase/supabase-js";
import { signInLocalIdentity } from "./helpers/admin";

const siteOrigin = process.env.BIODAYS_SITE_URL ?? "http://localhost:13000";

test("GitHub login is offered and provider errors never expose callback details", async ({
  page,
  request,
}) => {
  await page.goto("/admin/login");
  await expect(page.getByRole("button", { name: "使用 GitHub 账号登录" })).toBeVisible();
  await expect(page.getByRole("textbox")).toHaveCount(0);
  await expect(page.locator('input[type="password"]')).toHaveCount(0);
  const password = await request.post("/api/v1/admin/login", {
    headers: { Origin: siteOrigin },
    data: { username: "unused", password: "unused" },
  });
  expect(password.status()).toBe(410);
  const response = await request.get(
    "/auth/callback?error=access_denied&error_description=PRIVATE_PROVIDER_DETAIL&next=https://evil.test",
    { maxRedirects: 0 },
  );
  expect(response.status()).toBe(303);
  expect(response.headers().location).toBe(`${siteOrigin}/admin/login?error=github_failed`);
  expect(response.headers()["cache-control"]).toBe("no-store");
  expect(response.headers()["referrer-policy"]).toBe("no-referrer");
  await page.goto(response.headers().location);
  await expect(page.getByRole("alert").filter({ hasText: "GitHub 登录未完成" })).toBeVisible();
  await expect(page.locator("body")).not.toContainText("PRIVATE_PROVIDER_DETAIL");
});

test("an allowlisted GitHub identity can administer without legacy database membership", async ({
  page,
}) => {
  test.skip(
    !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "") ||
      !process.env.SUPABASE_SECRET_KEY,
    "Requires an isolated local Supabase project",
  );
  const client = createClient(
    process.env.SUPABASE_URL ?? "",
    process.env.SUPABASE_SECRET_KEY ?? "",
  );
  const userId = await signInLocalIdentity(page);
  const membership = await client.from("biodays_admins").delete().eq("user_id", userId);
  expect(membership.error).toBeNull();
  expect((await page.request.get("/api/v1/admin/me")).status()).toBe(200);
  expect((await page.request.get("/api/v1/admin/settings")).status()).toBe(200);
  await page.goto("/admin/feedback");
  await expect(page.getByRole("heading", { name: "反馈收件箱", exact: true })).toBeAttached();
});

test("forged metadata and legacy membership cannot bypass the GitHub allowlist", async ({
  page,
}) => {
  test.skip(
    !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "") ||
      !process.env.SUPABASE_SECRET_KEY,
    "Requires an isolated local Supabase project",
  );
  const client = createClient(
    process.env.SUPABASE_URL ?? "",
    process.env.SUPABASE_SECRET_KEY ?? "",
  );
  const email = `not-admin-${crypto.randomUUID()}@biodays.test`;
  const result = await client.auth.admin.createUser({
    email,
    email_confirm: true,
    user_metadata: {
      user_name: "liueic",
      provider_id: process.env.BIODAYS_ADMIN_GITHUB_IDS,
      role: "admin",
    },
  });
  expect(result.error).toBeNull();
  try {
    expect(result.data.user).not.toBeNull();
    const membership = await client
      .from("biodays_admins")
      .insert({ user_id: result.data.user?.id });
    expect(membership.error).toBeNull();
    await signInLocalIdentity(page, email);
    expect((await page.request.get("/api/v1/admin/me")).status()).toBe(403);
    expect((await page.request.get("/api/v1/admin/settings")).status()).toBe(403);
  } finally {
    if (result.data.user) await client.auth.admin.deleteUser(result.data.user.id);
  }
});

test("GitHub login rejects cross-site initiation and callback redirects", async ({ request }) => {
  const rejected = await request.post("/api/v1/admin/login/github", {
    headers: { Origin: "https://evil.test" },
  });
  expect(rejected.status()).toBe(403);
  const missing = await request.get("/auth/callback?next=https://evil.test", {
    maxRedirects: 0,
  });
  expect(missing.status()).toBe(303);
  expect(missing.headers().location).toBe(`${siteOrigin}/admin/login?error=github_failed`);
});

test("server-side GitHub login sets an HttpOnly PKCE cookie and refuses an invalid code", async ({
  request,
}) => {
  test.skip(
    !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "") ||
      !process.env.SUPABASE_SECRET_KEY,
    "Requires an isolated local Supabase project",
  );
  const response = await request.post("/api/v1/admin/login/github", {
    headers: { Origin: siteOrigin },
  });
  expect(response.status()).toBe(200);
  const url = new URL((await response.json()).data.url);
  expect(url.origin).toBe(new URL(process.env.SUPABASE_URL ?? "").origin);
  expect(url.pathname).toBe("/auth/v1/authorize");
  expect(url.searchParams.get("provider")).toBe("github");
  expect(url.searchParams.get("redirect_to")).toBe(`${siteOrigin}/auth/callback`);
  expect(url.searchParams.get("code_challenge")).toBeTruthy();
  expect(url.searchParams.get("code_challenge_method")?.toUpperCase()).toBe("S256");
  const cookie = response.headers()["set-cookie"];
  expect(cookie).toContain("code-verifier");
  expect(cookie).toContain("HttpOnly");
  const callback = await request.get("/auth/callback?code=invalid&next=https://evil.test", {
    maxRedirects: 0,
  });
  expect(callback.status()).toBe(303);
  expect(callback.headers().location).toBe(`${siteOrigin}/admin/login?error=github_failed`);
  expect((await request.get("/api/v1/admin/me")).status()).toBe(401);
});
