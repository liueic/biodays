import { expect, test } from "@playwright/test";

test("reading, archive, feedback privacy and invalid routes", async ({ page, request }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/story/Q1-01");
  await expect(
    page.getByRole("heading", { name: "终结“用嘴吸病毒和浓硫酸”的莽荒时代" }),
  ).toBeVisible();
  await expect(page.getByText("查看原始来源")).toHaveAttribute(
    "href",
    "https://www.eppendorf.com/",
  );
  await expect
    .poll(
      async () =>
        (await page.context().cookies()).find((c) => c.name === "biodays_read_v1")?.httpOnly,
    )
    .toBe(true);
  const random = await request.get("/api/v1/stories/random?exclude=Q1-01");
  expect(random.status()).toBe(200);
  expect((await random.json()).id).not.toBe("Q1-01");
  await page.getByRole("button", { name: "随机一篇(N)", exact: true }).click();
  await expect(page).not.toHaveURL(/\/story\/Q1-01$/);
  await page.goto("/archive");
  await expect(page.getByText("408", { exact: false }).first()).toBeVisible();
  await page.getByRole("searchbox", { name: "搜索故事" }).fill("Eppendorf");
  await expect(page.getByText("（已筛选）", { exact: false }).last()).toBeVisible();
  await expect(page.getByRole("link", { name: /终结“用嘴吸病毒/ })).toBeVisible();
  await page.getByRole("button", { name: "重置", exact: true }).click();
  // Window controls remain keyboard operable at both viewport sizes.
  const maximize = page.getByRole("button", { name: "最大化", exact: true });
  if (await maximize.count()) {
    await maximize.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "还原", exact: true })).toBeVisible();
  }
  await page.goto("/feedback?kind=issue&story_id=Q1-01");
  await expect(page.getByRole("heading", { name: /灵感|反馈/ }).first()).toBeAttached();
  expect(await page.locator('script[src*="umami"],script[src*="recorder"]').count()).toBe(0);
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute("content", /noindex/);
  const notFound = await request.get("/story/no-such-story");
  expect(notFound.status()).toBe(404);
  const sitemap = await request.get("/sitemap.xml");
  expect(((await sitemap.text()).match(/\/story\//g) || []).length).toBe(408);
  expect(errors).toEqual([]);
});

test("public API validation and isolated service failures", async ({ request, page }) => {
  const readiness = await request.get("/readyz");
  expect(readiness.status()).toBe(200);
  expect((await readiness.json()).content).toBe("ready");
  expect((await request.get("/api/v1/stories?day=2")).status()).toBe(400);
  expect((await request.get("/api/v1/today?tz=Asia%2FShanghai")).status()).toBe(400);
  expect((await request.get("/api/v1/stories?page_size=101")).status()).toBe(400);
  const r = await request.get("/api/v1/stories?view=summary&page_size=100");
  const payload = await r.json();
  expect(payload.meta.total).toBe(408);
  expect(payload.data).toHaveLength(100);
  expect(payload.data[0].content).toBeUndefined();
  expect((await request.post("/api/internal/mail/process")).status()).toBe(401);
  expect(
    (await request.post("/api/webhooks/resend", { data: {} })).status(),
  ).toBeGreaterThanOrEqual(400);
  await page.goto("/");
  await expect(page.locator("main").last()).toBeVisible();
  await page.goto("/admin/login");
  await expect(page.getByRole("button", { name: "使用 GitHub 账号登录" })).toBeVisible();
  await expect(page.getByRole("textbox")).toHaveCount(0);
});
