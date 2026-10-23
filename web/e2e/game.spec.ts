import { test, expect } from "@playwright/test";
import { openGameMenu } from "./helpers/game-menu";

test("missing 3D assets explain setup and preserve the room without a 2D fallback", async ({
  page,
}) => {
  const requests: string[] = [];
  page.on("request", (request) => requests.push(request.url()));
  await page.addInitScript(() =>
    localStorage.setItem("westeros-board-mode", "3d"),
  );
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "not imported" }),
  );
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("2D fallback");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(
    page.getByRole("heading", { name: "Нужны 3D-ассеты" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Повторить загрузку", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".region-piece")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "2D", exact: true }),
  ).toHaveCount(0);
  expect(
    requests.filter((url) => /GameBoard3D-|SkeletonUtils-/.test(url)),
  ).toEqual([]);
});

test("room, actual map pieces, original cards, soft timer, reload and tour", async ({
  page,
}) => {
  test.setTimeout(90000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Создать комнату" }),
  ).toBeVisible();
  await expect(page.locator("#turn-minutes")).toHaveValue("5");
  await expect(
    page.getByRole("checkbox", { name: /Жёсткий таймер/ }),
  ).not.toBeChecked();
  await page.getByLabel("Имя игрока", { exact: true }).fill("Browser player");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(
    page.getByRole("button", { name: "Начать партию" }),
  ).toBeVisible();
  await expect(page.getByTestId("game3d-board")).toHaveAttribute(
    "data-ready",
    "true",
    { timeout: 60000 },
  );
  await expect(page.locator("[data-region3d]")).toHaveCount(58);
  await expect(page.getByTestId("game3d-board")).toHaveAttribute(
    "data-piece-count",
    "27",
  );
  await expect(page.locator(".hand-cards button")).toHaveCount(7);
  const before = await page
    .locator(".hand-cards img")
    .first()
    .getAttribute("src");
  expect(before).toContain("/local-assets/");
  await page.locator(".hand-cards button").first().click();
  await expect(page.locator(".card-inspector-modal")).toBeVisible();
  await page.getByRole("button", { name: "Закрыть карту" }).click();
  await page.getByRole("button", { name: "Начать партию" }).click();
  await expect(page.locator(".phase-banner h2")).toHaveText(
    "Тайное планирование",
  );
  await expect(page.locator(".turn-clock")).toContainText("Мягкий лимит");
  await expect(page.locator(".turn-clock b")).toHaveText(/0[45]:\d\d/);
  const roomURL = page.url();
  await page.locator('[data-region3d="winterfell"]').click();
  await page.locator('.order-palette button[title="Поход"]').click();
  await expect(
    page.locator(".game3d-region.selected .game3d-order"),
  ).toBeVisible();
  const deadline = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    return (await fetch("/api/rooms/" + id).then((r) => r.json())).clock
      .deadline;
  });
  await page.reload();
  await expect(page.locator(".phase-banner h2")).toHaveText(
    "Тайное планирование",
  );
  expect(page.url()).toBe(roomURL);
  const after = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    return (await fetch("/api/rooms/" + id).then((r) => r.json())).clock
      .deadline;
  });
  expect(after).toBe(deadline);
  await openGameMenu(page);
  await page.getByRole("button", { name: "Обучение", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "Ваш дом и соперники" }),
  ).toBeVisible();
  const box = await page.locator(".tour-card").boundingBox();
  expect(box).not.toBeNull();
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.y + box!.height).toBeLessThanOrEqual(1000);
  await page.getByRole("button", { name: "Закрыть обучение" }).click();
  await page.screenshot({
    path: "test-results/playing-table.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Приказы готовы" }).click();
  await expect(page.getByRole("button", { name: "К действиям" })).toBeVisible();
  await page.getByRole("button", { name: "К действиям" }).click();
  await expect(page.locator(".phase-banner h2")).not.toHaveText(
    "Раскрытие приказов",
  );
  expect(errors).toEqual([]);
});

test("mobile welcome and settings fit the viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Создать комнату" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test("host can change the deadline and hard policy before start, not during a match", async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("Timer test");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(page.locator(".game-hud")).toBeVisible();
  await openGameMenu(page);
  await page
    .getByRole("button", { name: "Настройки комнаты", exact: true })
    .click();
  await page.locator("#turn-minutes").fill("8");
  await page.getByRole("checkbox", { name: /Жёсткий таймер/ }).check();
  await page.getByRole("button", { name: "Сохранить настройки" }).click();
  await expect(page.locator(".settings-modal")).not.toBeVisible();
  await page.reload();
  await page.getByRole("button", { name: "Начать партию" }).click();
  await expect(page.locator(".turn-clock")).toContainText("Жёсткий лимит");
  await openGameMenu(page);
  await page
    .getByRole("button", { name: "Настройки комнаты", exact: true })
    .click();
  await expect(page.locator("#turn-minutes")).toHaveValue("8");
  await expect(page.locator("#turn-minutes")).toBeDisabled();
  await expect(
    page.getByRole("checkbox", { name: /Жёсткий таймер/ }),
  ).toBeChecked();
});
