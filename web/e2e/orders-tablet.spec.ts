import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
});

test("orders explain effects on hover/focus and card hover never overlaps neighbours", async ({
  page,
}) => {
  test.setTimeout(90000);
  await page.goto("/?view=3d");
  await expect(
    page.getByRole("heading", { name: "Создать комнату" }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "3D-проба" })).toHaveCount(0);
  await page
    .getByLabel("Имя игрока", { exact: true })
    .fill("Order explanations");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(page.getByTestId("game3d-board")).toHaveAttribute(
    "data-ready",
    "true",
    { timeout: 60000 },
  );
  const card = page.locator(".hand-cards button").first();
  const before = (await card.boundingBox())!;
  await card.hover();
  const after = (await card.boundingBox())!;
  expect(Math.abs(before.width - after.width)).toBeLessThan(1);
  expect(Math.abs(before.x - after.x)).toBeLessThan(1);
  expect(
    await card.evaluate((element) => getComputedStyle(element).outlineStyle),
  ).toBe("solid");
  const next = (await page.locator(".hand-cards button").nth(1).boundingBox())!;
  expect(after.x + after.width).toBeLessThanOrEqual(next.x);
  await page.getByRole("button", { name: "Начать партию" }).click();
  const march = page.locator('.order-palette button[title="Поход"]');
  const help = page.locator(".order-help");
  await march.hover();
  const height = (await help.boundingBox())!.height;
  await expect(page.locator(".order-help")).toContainText(
    "перемещение подтверждается отдельно",
  );
  await expect(march).toHaveAttribute("aria-pressed", "false");
  const defend = page.locator('.order-palette button[title="Оборона +2 ★"]');
  // Descriptions must not move the palette, including the longer star hint.
  await page.mouse.move(0, 0);
  await defend.focus();
  await expect(page.locator(".order-help")).toContainText("Добавляет +2");
  await expect(page.locator(".order-help")).toContainText(
    "лимит зависит от Королевского двора",
  );
  expect((await help.boundingBox())!.height).toBe(height);
  const budget = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    return fetch(`/api/rooms/${id}`)
      .then((response) => response.json())
      .then((room) => room.orderBudget);
  });
  await expect(page.locator(".order-budget")).toContainText(
    `Особые приказы: осталось ${budget.starsAllowed} из ${budget.starsAllowed}`,
  );
  await defend.click();
  await expect(page.locator(".order-budget")).toContainText(
    `Особые приказы: осталось ${budget.starsAllowed - 1} из ${budget.starsAllowed}`,
  );
  // Use another real owned area to inspect a token already assigned elsewhere.
  await page.locator('[data-region3d="white-harbor"]').click();
  const used = page.locator('.order-palette button[data-used="true"]');
  await expect(used).toHaveCount(1);
  await expect(used).toHaveAttribute("aria-label", "Оборона +2 ★");
  await expect(used).toHaveAttribute("aria-disabled", "true");
  await expect(page.locator(".order-palette button")).toHaveCount(15);
  expect(
    Number(await used.evaluate((node) => getComputedStyle(node).opacity)),
  ).toBeLessThan(1);
  await used.hover();
  await used.focus();
  await expect(page.locator(".order-help")).toContainText("Уже назначен");
  await used.press("Enter");
  await expect(page.locator(".order-budget")).toContainText(
    `Особые приказы: осталось ${budget.starsAllowed - 1} из ${budget.starsAllowed}`,
  );
  await page.screenshot({
    path: test.info().outputPath("used-star-order.png"),
  });
});

test.describe("tablet touch", () => {
  test.use({
    viewport: { width: 1024, height: 768 },
    hasTouch: true,
    isMobile: true,
  });
  test("orders can be assigned, explained and removed with taps; cards open without hover", async ({
    page,
  }) => {
    await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
      route.fulfill({ status: 404, body: "UI-only test" }),
    );
    await page.goto("/");
    await page.getByLabel("Имя игрока", { exact: true }).fill("Tablet orders");
    await page.getByRole("button", { name: "Собрать малый совет" }).tap();
    await page.locator(".hand-cards button").first().tap();
    await expect(page.locator(".card-inspector-modal")).toBeVisible();
    const close = page.getByRole("button", {
      name: "Закрыть карту",
      exact: true,
    });
    expect((await close.boundingBox())!.width).toBeGreaterThanOrEqual(44);
    await close.tap();
    await page.getByRole("button", { name: "Начать партию" }).tap();
    const march = page.locator('.order-palette button[title="Поход"]');
    const box = (await march.boundingBox())!;
    expect(box.width).toBeGreaterThanOrEqual(44);
    expect(box.height).toBeGreaterThanOrEqual(44);
    await march.tap();
    await expect(march).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator(".order-help")).toContainText(
      "перемещение подтверждается отдельно",
    );
    await expect(page.locator(".order-help")).toBeInViewport();
    await expect(march).toHaveAttribute("aria-disabled", "false");
    await march.tap();
    await expect(march).toHaveAttribute("aria-pressed", "false");
    await page.locator('.order-palette button[title="Сбор власти ★"]').tap();
    await expect(page.locator(".order-help")).toContainText(
      "Особый приказ позволяет вместо власти собрать войска",
    );
    await page.screenshot({ path: "test-results/tablet-orders.png" });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  });

  test("one-finger pan and two-finger zoom do not select a region", async ({
    page,
  }) => {
    test.setTimeout(90000);
    await page.goto("/");
    await page.getByLabel("Имя игрока", { exact: true }).fill("Tablet camera");
    await page.getByRole("button", { name: "Собрать малый совет" }).tap();
    const board = page.getByTestId("game3d-board");
    await expect(board).toHaveAttribute("data-ready", "true", {
      timeout: 60000,
    });
    await page.waitForTimeout(1600);
    const box = (await page.locator("canvas").boundingBox())!;
    const session = await page.context().newCDPSession(page);
    const touch = async (
      type: string,
      points: { id: number; x: number; y: number }[],
    ) => {
      await session.send("Input.dispatchTouchEvent", {
        type,
        touchPoints: points,
      });
      await page.waitForTimeout(40);
    };
    const x = Math.round(box.x + box.width / 2),
      y = Math.round(box.y + box.height * 0.65);
    const target = await board.getAttribute("data-camera-target");
    await touch("touchStart", [{ id: 1, x, y }]);
    for (let i = 1; i <= 5; i++)
      await touch("touchMove", [{ id: 1, x: x + i * 12, y }]);
    await touch("touchEnd", []);
    await expect
      .poll(() => board.getAttribute("data-camera-target"))
      .not.toBe(target);
    await expect(page.locator(".inspector h2")).toHaveText("Винтерфелл");
    const distance = Number(await board.getAttribute("data-camera-distance"));
    await touch("touchStart", [
      { id: 1, x: x - 35, y },
      { id: 2, x: x + 35, y },
    ]);
    for (let i = 1; i <= 5; i++)
      await touch("touchMove", [
        { id: 1, x: x - 35 - i * 8, y },
        { id: 2, x: x + 35 + i * 8, y },
      ]);
    await touch("touchEnd", []);
    await expect
      .poll(async () =>
        Number(await board.getAttribute("data-camera-distance")),
      )
      .toBeLessThan(distance - 0.2);
    await expect(page.locator(".inspector h2")).toHaveText("Винтерфелл");
    await expect(board).toHaveAttribute("data-camera-polar", "0.785");
    await page.waitForTimeout(350);
    await page.locator('[data-region3d="white-harbor"]').tap();
    await expect(page.locator(".inspector h2")).toHaveText("Белая Гавань");
    await page.screenshot({ path: "test-results/tablet-board.png" });
    await session.detach();
  });
});
