import { expect, test } from "@playwright/test";

for (const viewport of [
  { width: 1600, height: 900 },
  { width: 1024, height: 768 },
  { width: 390, height: 844 },
  { width: 844, height: 390 },
]) {
  test(`only the form scrolls at ${viewport.width}×${viewport.height}`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.goto("/");
    const story = page.locator(".welcome-story");
    const form = page.locator(".create-room");
    await expect(story).toBeVisible();
    await expect(form).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    const before = (await story.boundingBox())!;
    expect(before.y).toBeGreaterThanOrEqual(0);
    expect(before.y + before.height).toBeLessThanOrEqual(viewport.height);
    expect(
      await form.evaluate(
        (element) => element.scrollHeight > element.clientHeight,
      ),
    ).toBe(true);
    const formBox = (await form.boundingBox())!;
    expect(formBox.y + formBox.height).toBeLessThanOrEqual(viewport.height);
    await page.mouse.move(
      formBox.x + formBox.width / 2,
      formBox.y + formBox.height / 2,
    );
    await page.mouse.wheel(0, 2000);
    await expect
      .poll(() => form.evaluate((element) => element.scrollTop))
      .toBeGreaterThan(0);
    const after = (await story.boundingBox())!;
    expect(after.y).toBeCloseTo(before.y, 1);
    expect(after.height).toBeCloseTo(before.height, 1);
    expect(await page.evaluate(() => window.scrollY)).toBe(0);
    expect(
      await page
        .locator("main.enhanced-welcome")
        .evaluate((element) => element.scrollTop),
    ).toBe(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await expect(
      page.getByRole("button", { name: "Собрать малый совет", exact: true }),
    ).toBeInViewport();
    await page.screenshot({
      path: testInfo.outputPath("fixed-introduction.png"),
    });
  });
}

test("Alegreya Cyrillic and Latin fonts are served locally", async ({
  page,
  baseURL,
}) => {
  const requests: string[] = [];
  page.on("request", (request) => {
    if (request.url().endsWith(".woff2")) requests.push(request.url());
  });
  await page.goto("/");
  await page.evaluate(async () => {
    await document.fonts.load(
      '500 18px "Alegreya Variable"',
      "Зима близко Winter is coming",
    );
    await document.fonts.ready;
  });
  expect(
    await page.evaluate(() =>
      document.fonts.check(
        '500 18px "Alegreya Variable"',
        "Зима близко Winter",
      ),
    ),
  ).toBe(true);
  expect(requests.length).toBeGreaterThan(0);
  for (const request of requests)
    expect(new URL(request).origin).toBe(new URL(baseURL!).origin);
});
