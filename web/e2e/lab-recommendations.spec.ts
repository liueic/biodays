import { expect, test } from "@playwright/test";

for (const theme of ["win", "mac"] as const) {
  test(`${theme} daily lab recommendations rotate, survive theme switching and open real articles`, async ({
    page,
    isMobile,
  }, testInfo) => {
    if (!isMobile) await page.setViewportSize({ width: 1280, height: 900 });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto("/");
    if (theme === "mac") {
      await page.getByRole("button", { name: "切换到 Mac", exact: true }).click();
    }
    const window = page.locator(".home-main-window .os-window");
    const storyTitle = await window.getByRole("heading", { level: 1 }).textContent();
    const recommendations = window.getByRole("region", { name: "实验室灵感", exact: true });
    const cards = recommendations.locator(".daily-lab-grid a");
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(0)).toHaveAttribute("href", /^\/lab\/protocols\/PROT-/);
    await expect(cards.nth(1)).toHaveAttribute("href", /^\/lab\/methods\/METH-/);
    expect(await recommendations.evaluate((el) => !!el.closest(".window-content"))).toBe(true);
    const initial = await cards.evaluateAll((links) =>
      links.map((link) => link.getAttribute("href")),
    );
    await page.reload();
    expect(
      await cards.evaluateAll((links) => links.map((link) => link.getAttribute("href"))),
    ).toEqual(initial);

    await recommendations.getByRole("button", { name: "换一组", exact: true }).click();
    const rotated = await cards.evaluateAll((links) =>
      links.map((link) => link.getAttribute("href")),
    );
    expect(rotated[0]).not.toBe(initial[0]);
    expect(rotated[1]).not.toBe(initial[1]);
    await expect(window.getByRole("heading", { level: 1 })).toHaveText(storyTitle ?? "");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    await recommendations.scrollIntoViewIfNeeded();
    await page.screenshot({ path: testInfo.outputPath(`${theme}-recommendations.png`) });

    await page
      .getByRole("button", {
        name: theme === "win" ? "切换到 Mac" : "切换到 Windows 98",
        exact: true,
      })
      .click();
    expect(
      await cards.evaluateAll((links) => links.map((link) => link.getAttribute("href"))),
    ).toEqual(rotated);
    await cards.nth(0).click();
    await expect(page).toHaveURL(new RegExp(`${rotated[0]}$`));
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    await page.goto("/");
    const techniqueHref = await cards.nth(1).getAttribute("href");
    await cards.nth(1).click();
    await expect(page).toHaveURL(new RegExp(`${techniqueHref}$`));
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    expect(errors).toEqual([]);
  });
}
