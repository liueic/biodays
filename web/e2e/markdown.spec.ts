import { expect, test } from "@playwright/test";

for (const theme of ["win", "mac"] as const) {
  test(`${theme} lab comparison table renders fully and stays inside the reading window`, async ({
    page,
    isMobile,
  }, testInfo) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto("/lab/methods/METH-03");
    if (theme === "mac") {
      await page.getByRole("button", { name: "切换到 Mac", exact: true }).click();
    }

    const body = page.locator(".markdown-content");
    const region = body.getByRole("region", { name: "表格（可横向滚动）", exact: true });
    const table = region.getByRole("table");
    await expect(table.getByRole("columnheader")).toHaveText([
      "策略方案",
      "改造核心机制",
      "特异性提升幅度",
      "潜在代价与局限",
    ]);
    await expect(table.locator("tbody tr")).toHaveCount(3);
    await expect(table.getByRole("cell")).toHaveCount(12);
    await expect(table.getByRole("cell").first()).toHaveText("SpCas9-HF1（高保真变体 1）");
    await expect(table.getByRole("cell").last()).toContainText("空间内找到两侧背靠背的 PAM 位点");
    await expect(table.locator(".katex")).toHaveCount(2);
    await expect(body.locator(".katex-error")).toHaveCount(0);
    await expect(body.locator("h3").first()).toHaveCSS("font-weight", "700");
    await expect(body.locator("ol").first()).toHaveCSS("list-style-type", "decimal");
    await expect(region).toHaveCSS("overflow-x", "auto");
    await expect(table.getByRole("columnheader").first()).toHaveCSS("border-top-style", "solid");

    await region.scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const dimensions = await region.evaluate((el) => ({
      width: el.clientWidth,
      scrollWidth: el.scrollWidth,
      parentWidth: el.parentElement?.clientWidth ?? 0,
    }));
    expect(dimensions.width).toBeLessThanOrEqual(dimensions.parentWidth);
    if (isMobile) {
      expect(dimensions.scrollWidth).toBeGreaterThan(dimensions.width);
      await region.evaluate((el) => {
        el.scrollLeft = el.scrollWidth;
      });
      expect(await region.evaluate((el) => el.scrollLeft)).toBeGreaterThan(0);
      const lastCell = await table.getByRole("cell").last().boundingBox();
      const bounds = await region.boundingBox();
      expect(lastCell).not.toBeNull();
      expect(bounds).not.toBeNull();
      expect((lastCell?.x ?? 0) + (lastCell?.width ?? 0)).toBeLessThanOrEqual(
        (bounds?.x ?? 0) + (bounds?.width ?? 0) + 1,
      );
      await region.evaluate((el) => {
        el.scrollLeft = 0;
      });
    }
    await page.screenshot({ path: testInfo.outputPath(`${theme}-markdown-table.png`) });
    expect(errors).toEqual([]);
  });
}
