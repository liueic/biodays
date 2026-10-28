import { expect, type Locator, type Page, test } from "@playwright/test";

async function switchTheme(page: Page, theme: "win" | "mac") {
  if ((await page.locator("html").getAttribute("data-os")) === theme) return;
  await page
    .getByRole("button", {
      name: theme === "mac" ? "切换到 Mac" : "切换到 Windows 98",
      exact: true,
    })
    .click();
  await expect(page.locator("html")).toHaveAttribute("data-os", theme);
}

async function expectInsideWorkspace(page: Page, window: Locator) {
  // A viewport resize is followed by a ResizeObserver and a React position update.
  await expect
    .poll(async () => {
      const bounds = await window.boundingBox();
      const workspace = await page.locator(".desktop-workspace").boundingBox();
      if (!bounds || !workspace) return false;
      return (
        bounds.x >= workspace.x - 1 &&
        bounds.y >= workspace.y - 1 &&
        bounds.x + bounds.width <= workspace.x + workspace.width + 1 &&
        bounds.y + bounds.height <= workspace.y + workspace.height + 1
      );
    })
    .toBe(true);
  const bounds = await window.boundingBox();
  const workspace = await page.locator(".desktop-workspace").boundingBox();
  if (!bounds || !workspace) throw new Error("Missing desktop geometry");
  expect(bounds.x).toBeGreaterThanOrEqual(workspace.x - 1);
  expect(bounds.y).toBeGreaterThanOrEqual(workspace.y - 1);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(workspace.x + workspace.width + 1);
  expect(bounds.y + bounds.height).toBeLessThanOrEqual(workspace.y + workspace.height + 1);
  expect(await page.evaluate(() => document.documentElement.scrollHeight <= innerHeight)).toBe(
    true,
  );
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

async function dragTitle(page: Page, title: Locator, x: number, y: number) {
  const bounds = await title.boundingBox();
  if (!bounds) throw new Error("Missing title bar");
  const startX = bounds.x + bounds.width / 2;
  const startY = bounds.y + bounds.height / 2;
  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(startX + x, startY + y, { steps: 10 });
  await page.mouse.up();
}

test("both skins keep readable windows within desktop viewports", async ({
  page,
  isMobile,
}, testInfo) => {
  test.skip(isMobile, "Precise-pointer desktop geometry.");
  await page.goto("/");
  for (const width of [1024, 1280, 1440]) {
    await page.setViewportSize({ width, height: 720 });
    for (const theme of ["win", "mac"] as const) {
      await switchTheme(page, theme);
      const window = page.locator(".home-main-window .os-window");
      await expect(window).toBeVisible();
      const icons = page.getByRole("navigation", { name: "桌面图标" });
      await expect(icons.getByRole("button")).toHaveCount(5);
      await expect(page.locator("#home-lab-title")).toHaveCount(0);
      await expect(page.locator(".home-recent-window")).toHaveCount(width >= 1200 ? 1 : 0);
      await expect
        .poll(async () => (await window.boundingBox())?.width ?? 0)
        .toBeGreaterThanOrEqual(640);
      expect((await window.boundingBox())?.width).toBeLessThanOrEqual(720);
      const iconBounds = await icons.boundingBox();
      const windowBounds = await window.boundingBox();
      if (!iconBounds || !windowBounds) throw new Error("Missing icons");
      if (theme === "win") expect(iconBounds.x + iconBounds.width).toBeLessThan(windowBounds.x);
      else expect(iconBounds.x).toBeGreaterThan(windowBounds.x + windowBounds.width);
      expect(
        await window.locator(".window-content").evaluate((el) => getComputedStyle(el).overflowY),
      ).toBe("auto");
      await expectInsideWorkspace(page, window);
      if (width === 1280)
        await page.screenshot({ path: testInfo.outputPath(`${theme}-desktop.png`) });
    }
  }
});

test("desktop windows hide, reopen and stay reachable after dragging and resizing", async ({
  page,
  isMobile,
}) => {
  test.skip(isMobile, "Desktop window lifecycle and dragging.");
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto("/");
  const window = page.locator(".home-main-window .os-window");
  await window.getByRole("button", { name: "最小化", exact: true }).click();
  await expect(window).toBeHidden();
  await page
    .locator(".desktop-taskbar")
    .getByRole("button", { name: /每日提示/ })
    .click();
  await expect(window).toBeVisible();
  await window.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(window).toBeHidden();
  await page
    .getByRole("navigation", { name: "桌面图标" })
    .getByRole("button", { name: "今日故事" })
    .focus();
  await page.keyboard.press("Enter");
  await expect(window).toBeVisible();

  const recent = page.locator(".home-recent-window");
  await recent.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(recent).toHaveCount(0);
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await page
    .locator("#desktop-start-menu")
    .getByRole("button", { name: "最近归档", exact: true })
    .click();
  await expect(recent).toBeVisible();

  for (const theme of ["win", "mac"] as const) {
    await switchTheme(page, theme);
    const title = window.locator(
      theme === "win" ? ".win98-title > div:first-of-type" : ".mac-titlebar-title",
    );
    await dragTitle(page, title, 1000, 1000);
    await expectInsideWorkspace(page, window);
    await page.setViewportSize({ width: 1024, height: 600 });
    await expect.poll(async () => (await window.boundingBox())?.x ?? -1).toBeGreaterThanOrEqual(0);
    await expectInsideWorkspace(page, window);
    await page.setViewportSize({ width: 900, height: 720 });
    await expect.poll(() => window.evaluate((el) => getComputedStyle(el).translate)).toBe("0px");
    await page.setViewportSize({ width: 1280, height: 720 });
    await expect(recent).toBeVisible();
    await expectInsideWorkspace(page, window);
  }
});

test("long documents scroll inside their window in both skins", async ({ page, isMobile }) => {
  test.skip(isMobile, "Desktop scrolling boundary.");
  await page.setViewportSize({ width: 1280, height: 600 });
  for (const theme of ["win", "mac"] as const) {
    await page.goto("/story/Q1-01");
    await switchTheme(page, theme);
    const window = page.locator(".os-window");
    await expectInsideWorkspace(page, window);
    const content = window.locator(".window-content");
    expect(await content.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
    await content.hover();
    await page.mouse.wheel(0, 2000);
    await expect.poll(() => content.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
    expect(await page.evaluate(() => scrollY)).toBe(0);
    await window
      .getByRole("button", { name: theme === "win" ? "最大化" : "缩放", exact: true })
      .click();
    await expect(window).toHaveClass(/os-maximized/);
    await expect(window.getByRole("button", { name: "还原", exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollHeight <= innerHeight)).toBe(
      true,
    );
    await window.getByRole("button", { name: "还原", exact: true }).click();
    await expectInsideWorkspace(page, window);
  }
});

test("compact screens use one undraggable document window", async ({
  page,
  isMobile,
}, testInfo) => {
  test.skip(isMobile, "Intermediate desktop breakpoint.");
  await page.setViewportSize({ width: 900, height: 720 });
  await page.goto("/");
  for (const theme of ["win", "mac"] as const) {
    await switchTheme(page, theme);
    await expect(page.locator(".os-desktop-icons:visible")).toHaveCount(0);
    await expect(page.locator(".home-recent-window")).toHaveCount(0);
    const window = page.locator(".home-main-window .os-window");
    const before = await window.boundingBox();
    await dragTitle(
      page,
      window.locator(theme === "win" ? ".win98-title > div:first-of-type" : ".mac-titlebar-title"),
      80,
      40,
    );
    expect(await window.boundingBox()).toEqual(before);
  }
  await expect(page.getByRole("button", { name: "File", exact: true })).toBeVisible();
  await expect(page.locator(".mac-menu-edit")).toBeHidden();
  await expect(page.locator(".mac-menu-view")).toBeHidden();
  await expect(page.locator(".mac-menu-special")).toBeHidden();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("compact-mac.png") });
});

test("mobile and large touch screens retain natural scrolling and all destinations", async ({
  page,
  isMobile,
}, testInfo) => {
  test.skip(!isMobile, "Touch-only layout.");
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: 720 });
    await page.goto("/story/Q1-01");
    for (const theme of ["win", "mac"] as const) {
      await switchTheme(page, theme);
      await expect(page.locator(".os-desktop-icons:visible")).toHaveCount(0);
      await expect(page.locator(".home-recent-window")).toHaveCount(0);
      const content = page.locator(".window-content");
      expect(await content.evaluate((el) => getComputedStyle(el).overflowY)).toBe("visible");
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true,
      );
      await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
      await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(0);
      await page.evaluate(() => window.scrollTo(0, 0));
      if (theme === "mac") {
        await page.getByRole("button", { name: "File", exact: true }).click();
        for (const name of ["今日故事", "归档浏览", "实验室手册", "灵感信箱", "关于本站"]) {
          await expect(
            page.locator("#mac-file-menu").getByRole("link", { name, exact: true }),
          ).toBeVisible();
        }
        await page.keyboard.press("Escape");
      }
    }
    if (width === 390) await page.screenshot({ path: testInfo.outputPath("mobile-mac.png") });
  }
});

test("compact system menus expose auxiliary commands and let random reading retry", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 720 });
  await page.goto("/");
  for (const theme of ["win", "mac"] as const) {
    await switchTheme(page, theme);
    let attempts = 0;
    await page.route("**/api/v1/stories/random*", (route) => {
      attempts += 1;
      if (attempts === 1) return route.fulfill({ status: 503, json: { error: "Unavailable" } });
      return route.continue();
    });
    await page
      .getByRole("button", { name: theme === "win" ? "开始" : "File", exact: true })
      .click();
    const menu = page.locator(theme === "win" ? "#desktop-start-menu" : "#mac-file-menu");
    await menu.getByRole("button", { name: "随机一篇", exact: true }).click();
    const retry = menu.getByRole("button", { name: "获取失败，点击重试", exact: true });
    await expect(retry).toBeVisible();
    await retry.click();
    await expect(page).toHaveURL(/\/story\//);
    await expect(menu).toHaveCount(0);
    await page.unroute("**/api/v1/stories/random*");
    await page.goto("/");
  }
});
