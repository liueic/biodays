import { expect, test } from "@playwright/test";

const macAgent =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36";
const winAgent =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36";

test("switch icons preserve input, preference and navigation", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/admin/login");
  const email = page.getByLabel("邮箱", { exact: true });
  await email.fill("draft@biodays.test");
  await page.getByLabel("密码", { exact: true }).fill("unsent-password");
  const inputNode = await email.elementHandle();
  await page.getByRole("button", { name: "切换到 Mac", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-os", "mac");
  await expect(page.locator(".mac-menu-bar")).toBeVisible();
  await expect(email).toHaveValue("draft@biodays.test");
  expect(await inputNode?.evaluate((element) => element.isConnected)).toBe(true);
  await page.getByRole("button", { name: "切换到 Windows 98", exact: true }).click();
  await expect(page.locator(".desktop-taskbar")).toBeVisible();
  await expect(email).toHaveValue("draft@biodays.test");
  await expect(page.getByLabel("密码", { exact: true })).toHaveValue("unsent-password");
  await page.getByRole("button", { name: "切换到 Mac", exact: true }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-os", "mac");
  await page.getByRole("button", { name: "File", exact: true }).click();
  await page.getByRole("link", { name: "实验室手册", exact: true }).click();
  await expect(page).toHaveURL(/\/lab$/);
  await expect(page.getByRole("heading", { name: "实验室手册", exact: true })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-os", "mac");
  await page.goto("/archive");
  const search = page.getByRole("searchbox", { name: "搜索故事" });
  await search.fill("Eppendorf");
  await expect(page.getByRole("link", { name: /终结“用嘴吸病毒/ })).toBeVisible();
  await page.getByRole("button", { name: "切换到 Windows 98", exact: true }).click();
  await expect(search).toHaveValue("Eppendorf");
  expect(errors).toEqual([]);
});

test("both desktop icon sets expose a working theme switch", async ({ page }, testInfo) => {
  await page.goto("/");
  await page
    .locator(".desktop-taskbar")
    .getByRole("button", { name: "切换到 Mac", exact: true })
    .click();
  await page
    .getByRole("navigation", { name: "桌面图标" })
    .getByRole("button", { name: "切换到 Windows 98", exact: true })
    .dblclick();
  await expect(page.locator("html")).toHaveAttribute("data-os", "win");
  await page.screenshot({ path: testInfo.outputPath("win-home.png"), fullPage: true });
  await page
    .getByRole("navigation", { name: "桌面图标" })
    .getByRole("button", { name: "切换到 Mac", exact: true })
    .dblclick();
  await expect(page.locator("html")).toHaveAttribute("data-os", "mac");
  await expect(
    page
      .getByRole("navigation", { name: "桌面图标" })
      .getByRole("button", { name: "实验室手册", exact: true }),
  ).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("mac-home.png"), fullPage: true });
});

test("Mac window controls preserve content and can reopen a closed window", async ({
  page,
  isMobile,
}) => {
  await page.goto("/admin/login");
  await page.getByRole("button", { name: "切换到 Mac", exact: true }).click();
  await page.getByLabel("邮箱", { exact: true }).fill("shade@biodays.test");
  await page.getByRole("button", { name: "折叠", exact: true }).click();
  await expect(page.getByLabel("邮箱", { exact: true })).toBeHidden();
  await page.getByRole("button", { name: "展开", exact: true }).click();
  await expect(page.getByLabel("邮箱", { exact: true })).toHaveValue("shade@biodays.test");
  if (!isMobile) {
    const title = page.locator(".os-window .mac-titlebar-title");
    const before = await page.locator(".os-window").boundingBox();
    const handle = await title.boundingBox();
    if (!before || !handle) throw new Error("Missing desktop window bounds");
    await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2);
    await page.mouse.down();
    await page.mouse.move(handle.x + handle.width / 2 + 40, handle.y + handle.height / 2 + 20, {
      steps: 8,
    });
    await expect(page.locator(".mac-drag-outline")).toBeVisible();
    await page.mouse.up();
    await expect(page.locator(".mac-drag-outline")).toHaveCount(0);
    const after = await page.locator(".os-window").boundingBox();
    expect(after?.x).toBeGreaterThan(before.x);
  }
  await page.getByRole("button", { name: "缩放", exact: true }).click();
  await expect(page.locator(".os-window")).toHaveClass(/os-maximized/);
  await page.getByRole("button", { name: "切换到 Windows 98", exact: true }).click();
  await expect(page.locator(".os-window")).toHaveClass(/os-maximized/);
  await page.getByRole("button", { name: "还原", exact: true }).click();
  await expect(page.locator(".os-window")).not.toHaveClass(/os-maximized/);
  await page.getByRole("button", { name: "最小化", exact: true }).click();
  await expect(page.locator(".os-window")).toBeHidden();
  await page.getByRole("button", { name: "切换到 Mac", exact: true }).click();
  await page.getByRole("button", { name: "应用程序菜单", exact: true }).click();
  await page
    .locator(".mac-window-menu")
    .getByRole("button", { name: /管理员登录/ })
    .click();
  await expect(page.locator(".os-window")).toBeVisible();
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(page.locator(".os-window")).toBeHidden();
  await page.getByRole("button", { name: "File", exact: true }).click();
  await page.getByRole("link", { name: "今日故事", exact: true }).click();
  await expect(page.locator(".os-window")).toBeVisible();
});

test("OS detection and both skins work before React hydrates", async ({ browser, baseURL }) => {
  for (const [userAgent, theme] of [
    [macAgent, "mac"],
    [winAgent, "win"],
  ] as const) {
    const context = await browser.newContext({ userAgent, viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    // The synchronous head script still runs, while application JS never hydrates.
    await page.route("**/_next/**/*.js*", (route) => route.abort());
    await page.goto(`${baseURL}/story/Q1-01`, { waitUntil: "domcontentloaded" });
    await expect(page.locator("html")).toHaveAttribute("data-os", theme);
    await expect(page.getByRole("heading", { name: /终结“用嘴吸病毒/ })).toBeVisible();
    await expect(
      page.locator(theme === "mac" ? ".mac-menu-bar" : ".desktop-taskbar"),
    ).toBeVisible();
    await expect(page.locator(theme === "mac" ? ".desktop-taskbar" : ".mac-menu-bar")).toBeHidden();
    const window = page.locator(".os-window");
    await expect(window.locator(theme === "mac" ? ".mac-titlebar" : ".win98-title")).toBeVisible();
    await expect(window.locator(theme === "mac" ? ".win98-title" : ".mac-titlebar")).toBeHidden();
    await page.unroute("**/_next/**/*.js*");
    await page.reload();
    const switchName = theme === "mac" ? "切换到 Windows 98" : "切换到 Mac";
    await page.getByRole("button", { name: switchName, exact: true }).click();
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-os", theme === "mac" ? "win" : "mac");
    expect(errors).toEqual([]);
    await context.close();
  }
});
