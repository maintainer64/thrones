import { expect, test } from "@playwright/test";

test("game HUD, architectural models, collapsible hand and server-authoritative orders", async ({
  page,
}) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  const models = new Set<string>();
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("response", (response) => {
    if (
      /landmark-(castle|stronghold|port)\.glb/.test(response.url()) &&
      response.ok()
    )
      models.add(response.url());
  });
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("HUD player");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  const board = page.getByTestId("game3d-board");
  await expect(board).toHaveAttribute("data-ready", "true", { timeout: 60000 });
  await expect(board).toHaveAttribute("data-landmark-count", "28");
  expect(models.size).toBe(3);
  await expect(page.locator(".house-hand-dock .hand-cards button")).toHaveCount(
    7,
  );
  const boardBox = (await page.locator("canvas").boundingBox())!;
  const handBox = (await page.locator(".house-hand-dock").boundingBox())!;
  const cardBox = (await page
    .locator(".hand-cards .house-card")
    .first()
    .boundingBox())!;
  expect(cardBox.y + cardBox.height).toBeLessThanOrEqual(1000);
  expect(boardBox.y + boardBox.height).toBeLessThanOrEqual(handBox.y);
  await page.screenshot({ path: "test-results/hud-table.png" });
  await page.getByLabel("Свернуть руку карт", { exact: true }).click();
  await expect(page.locator(".game-hud")).toHaveAttribute(
    "data-hand-open",
    "false",
  );
  await expect
    .poll(async () => (await page.locator("canvas").boundingBox())!.height)
    .toBeGreaterThan(boardBox.height + 100);
  await page.getByLabel("Показать руку карт", { exact: true }).click();
  await page
    .getByRole("button", { name: "Панель выбранной области", exact: true })
    .click();
  await expect(page.locator(".inspector")).not.toBeVisible();
  await expect
    .poll(async () => (await page.locator("canvas").boundingBox())!.width)
    .toBeGreaterThan(boardBox.width + 200);
  await page
    .getByRole("button", { name: "Панель выбранной области", exact: true })
    .click();
  await page.getByRole("button", { name: "Начать партию" }).click();
  await page.locator('[data-region3d="the-shivering-sea"]').click();
  await expect(page.locator(".inspector h2")).toHaveText("Дрожащее море");
  for (const token of await page
    .locator('.order-palette button[title^="Сбор власти"]')
    .all())
    await expect(token).toHaveAttribute("aria-disabled", "true");
  const options = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    const room = await fetch(`/api/rooms/${id}`).then((response) =>
      response.json(),
    );
    return room.availableOrders["the-shivering-sea"] as string[];
  });
  // All physical tokens stay visible; unavailable/used tokens are dimmed.
  await expect(page.locator(".order-palette button")).toHaveCount(15);
  expect(
    await page.locator('.order-palette button[aria-disabled="false"]').count(),
  ).toBe(options.length);
  await page.locator('.order-palette button[title="Поход"]').click();
  await expect(
    page.locator('.order-palette button[title="Поход"]'),
  ).toHaveAttribute("aria-pressed", "true");
  await page.locator('[data-region3d="winterfell"]').click();
  await expect(
    page.locator('.order-palette button[aria-label="Поход"]'),
  ).toHaveAttribute("aria-disabled", "true");
  await page.screenshot({ path: "test-results/hud-orders.png" });
  await page
    .getByRole("button", { name: "Панель дорожек влияния", exact: true })
    .click();
  const tracks = page.getByRole("complementary", {
    name: "Треки влияния",
    exact: true,
  });
  await expect(tracks).toBeVisible();
  await expect(page.locator(".inspector")).not.toBeVisible();
  await expect(tracks.locator(".influence-columns section")).toHaveCount(3);
  const actual = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    return fetch(`/api/rooms/${id}`).then((r) => r.json());
  });
  for (const [index, track] of [
    "ironThrone",
    "fiefdoms",
    "kingsCourt",
  ].entries()) {
    const column = tracks.locator(".influence-columns section").nth(index);
    await expect(column.locator("li")).toHaveCount(6);
    expect(
      await column
        .locator("h3")
        .evaluate((heading) => heading.scrollWidth <= heading.clientWidth),
    ).toBe(true);
    expect(
      await column
        .locator("li img")
        .evaluateAll((images) =>
          images.map((image) =>
            (image as HTMLImageElement).src
              .split("/")
              .at(-1)!
              .replace(".svg", ""),
          ),
        ),
    ).toEqual(actual.match.tracks[track]);
  }
  await expect(page.locator(".phase-steps [aria-current]")).toHaveText(
    "Тайное планирование",
  );
  const panelBox = (await tracks.boundingBox())!;
  const canvasBox = (await page.locator("canvas").boundingBox())!;
  expect(canvasBox.x + canvasBox.width).toBeLessThanOrEqual(panelBox.x);
  await page
    .getByRole("button", { name: "Панель великих домов", exact: true })
    .click();
  await page.screenshot({ path: "test-results/strategy-desktop.png" });
  await page
    .getByRole("button", { name: "Скрыть дорожки влияния", exact: true })
    .click();
  await expect(page.locator(".inspector")).toBeVisible();
  expect(errors).toEqual([]);
});

test.describe("tablet HUD", () => {
  test.use({
    viewport: { width: 1024, height: 768 },
    hasTouch: true,
    isMobile: true,
  });
  test("hand and panels remain accessible with taps", async ({ page }) => {
    await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
      route.fulfill({ status: 404, body: "HUD interaction test" }),
    );
    await page.addInitScript(() =>
      localStorage.setItem("westeros-tour-v2", "true"),
    );
    await page.goto("/");
    await page.getByLabel("Имя игрока", { exact: true }).fill("Tablet HUD");
    await page.getByRole("button", { name: "Собрать малый совет" }).tap();
    await expect(page.locator(".houses")).not.toBeVisible();
    await page
      .getByRole("button", { name: "Панель великих домов", exact: true })
      .tap();
    await expect(page.locator(".houses")).toBeVisible();
    await page
      .getByRole("button", { name: "Панель великих домов", exact: true })
      .tap();
    await page.locator(".hand-cards button").last().tap();
    await expect(page.locator(".card-inspector-modal")).toBeVisible();
    await page
      .getByRole("button", { name: "Закрыть карту", exact: true })
      .tap();
    await page.getByLabel("Свернуть руку карт", { exact: true }).tap();
    await expect(page.locator(".game-hud")).toHaveAttribute(
      "data-hand-open",
      "false",
    );
    await page.getByLabel("Показать руку карт", { exact: true }).tap();
    await expect(page.locator(".game-hud")).toHaveAttribute(
      "data-hand-open",
      "true",
    );
    await expect(page.locator(".hand-cards button").first()).toBeInViewport();
    const cardBox = (await page
      .locator(".hand-cards .house-card")
      .first()
      .boundingBox())!;
    expect(cardBox.y + cardBox.height).toBeLessThanOrEqual(768);
    await page
      .getByRole("button", { name: "Панель дорожек влияния", exact: true })
      .tap();
    await expect(
      page.locator(".influence-panel .influence-columns section"),
    ).toHaveCount(3);
    const influenceBox = (await page
      .locator(".influence-panel")
      .boundingBox())!;
    expect(influenceBox.x + influenceBox.width).toBeLessThanOrEqual(1024);
    await page
      .getByRole("button", { name: "Скрыть дорожки влияния", exact: true })
      .tap();
    await page.screenshot({ path: "test-results/hud-tablet.png" });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  });
});
