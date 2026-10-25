import { expect, test } from "@playwright/test";
import { openGameMenu } from "./helpers/game-menu";

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "menu-only test" }),
  );
});

test("no masthead or prototype banner; the cog opens settings and Escape restores focus", async ({
  page,
}, testInfo) => {
  await page.goto("/");
  await expect(page.locator("header, .prototype-note")).toHaveCount(0);
  await page.getByLabel("Имя игрока", { exact: true }).fill("Menu player");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(page.locator(".game-hud")).toBeVisible();
  expect((await page.locator(".phase-banner").boundingBox())!.y).toBe(0);
  await expect(
    page.getByText("Ассеты вашей локальной игры", { exact: true }),
  ).toHaveCount(0);
  await openGameMenu(page);
  await expect(page.getByTestId("language-switch")).toBeFocused();
  await page.screenshot({ path: testInfo.outputPath("settings-menu.png") });
  await page.keyboard.press("Escape");
  await expect(page.locator(".game-menu-panel")).toHaveCount(0);
  await expect(page.getByTestId("game-menu-toggle")).toBeFocused();
  await openGameMenu(page);
  await page
    .getByRole("button", { name: "Настройки комнаты", exact: true })
    .click();
  await expect(page.locator(".settings-modal")).toBeVisible();
  await expect(page.locator(".game-menu-panel")).toHaveCount(0);
  await page.locator("#turn-minutes").fill("6");
  await page.getByRole("button", { name: "Сохранить настройки" }).click();
  await page
    .getByRole("button", { name: "Начать партию", exact: true })
    .click();
  await expect(page.locator(".phase-status .turn-clock b")).toHaveText(
    /\d{2}:\d{2}/,
  );
});

test("preferences persist and navigation stays available inside the settings menu", async ({
  page,
}) => {
  await page.goto("/");
  await openGameMenu(page);
  await page.getByRole("button", { name: "Звук", exact: true }).click();
  await openGameMenu(page);
  await page.getByRole("button", { name: "Анимации", exact: true }).click();
  await page.reload();
  await expect(page.locator(".app")).toHaveClass(/no-motion/);
  await openGameMenu(page);
  await expect(
    page.getByRole("button", { name: "Звук", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("button", { name: "Анимации", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
  await page
    .getByRole("button", { name: "Правила и дополнения", exact: true })
    .click();
  await expect(page.locator("main.rules")).toBeVisible();
  await openGameMenu(page);
  await page.getByRole("button", { name: "Поле и дома", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Создать комнату", exact: true }),
  ).toBeVisible();
});

test.describe("touch settings", () => {
  test.use({ hasTouch: true, isMobile: true });
  test("touch menu fits a phone, preserves tap targets and closes on outside tap", async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/");
    await page.getByTestId("game-menu-toggle").tap();
    const panel = page.locator(".game-menu-panel");
    await expect(panel).toBeVisible();
    const box = (await panel.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
    expect(box.y + box.height).toBeLessThanOrEqual(844);
    for (const button of await panel.locator("button").all())
      expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await page.screenshot({ path: testInfo.outputPath("settings-phone.png") });
    await page.touchscreen.tap(8, box.y + 12);
    await expect(panel).toHaveCount(0);
  });
});
