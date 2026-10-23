import { expect, type Page } from "@playwright/test";

export async function openGameMenu(page: Page) {
  const trigger = page.getByTestId("game-menu-toggle");
  if ((await trigger.getAttribute("aria-expanded")) !== "true")
    await trigger.click();
  await expect(page.locator(".game-menu-panel")).toBeVisible();
}

export async function switchLanguage(page: Page) {
  await openGameMenu(page);
  await page.getByTestId("language-switch").click();
}
