import { test, expect } from "@playwright/test";
import { openGameMenu, switchLanguage } from "./helpers/game-menu";

test("RU/EN switches live cards, board, tutorial and server logs without restarting the room", async ({
  page,
}) => {
  test.setTimeout(90000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Создать комнату" }),
  ).toBeVisible();
  await switchLanguage(page);
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(
    page.getByRole("heading", { name: "Create room" }),
  ).toBeVisible();
  await page.getByLabel("Player name", { exact: true }).fill("Русский игрок");
  await page.getByRole("button", { name: "Gather the Small Council" }).click();
  await expect(
    page.getByRole("button", { name: "Start game", exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId("game3d-board")).toHaveAttribute(
    "data-ready",
    "true",
    { timeout: 60000 },
  );
  await expect(page.locator("[data-region3d='winterfell']")).toContainText(
    "Winterfell",
  );
  await expect(page.locator(".events")).toContainText("Room created.");
  await expect(page.locator(".hand-cards")).toContainText("Eddard Stark");
  await page
    .locator(".hand-cards button")
    .filter({ hasText: "Eddard Stark" })
    .click();
  await expect(page.locator(".card-inspector-modal")).toContainText(
    "No special text ability",
  );
  await page.getByRole("button", { name: "Close card", exact: true }).click();
  await page.getByRole("button", { name: "Start game", exact: true }).click();
  await expect(page.locator(".phase-banner h2")).toHaveText("Secret planning");
  const roomURL = page.url();
  const snapshot = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    return fetch(`/api/rooms/${id}`).then((r) => r.json());
  });
  await switchLanguage(page);
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
  await expect(page.locator(".phase-banner h2")).toHaveText(
    "Тайное планирование",
  );
  await expect(page.locator(".hand-cards")).toContainText("Эддард Старк");
  await expect(page.locator("[data-region3d='winterfell']")).toContainText(
    "Винтерфелл",
  );
  await expect(page.locator(".events")).toContainText(
    "Начато тайное планирование",
  );
  expect(page.url()).toBe(roomURL);
  const after = await page.evaluate(async () => {
    const id = new URLSearchParams(location.search).get("room");
    return fetch(`/api/rooms/${id}`).then((r) => r.json());
  });
  expect(after.revision).toBe(snapshot.revision);
  expect(after.clock.deadline).toBe(snapshot.clock.deadline);
  expect(after.seats[0].name).toBe("Русский игрок");
  await switchLanguage(page);
  await openGameMenu(page);
  await page.getByRole("button", { name: "Tutorial", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "Your house and opponents" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Close tutorial" }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.locator(".phase-banner h2")).toHaveText("Secret planning");
  expect(page.url()).toBe(roomURL);
  await expect(page.getByTestId("game3d-board")).toHaveAttribute(
    "data-ready",
    "true",
    { timeout: 60000 },
  );
  await page.screenshot({ path: "test-results/localization-en-desktop.png" });
  expect(errors).toEqual([]);
});

test("English is available from a link and the switch remains usable on a tablet", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto("/?lang=en");
  await expect(
    page.getByRole("heading", { name: "Create room" }),
  ).toBeVisible();
  const button = page.getByTestId("language-switch");
  await openGameMenu(page);
  await expect(button).toBeInViewport();
  await button.click();
  await expect(
    page.getByRole("heading", { name: "Создать комнату" }),
  ).toBeVisible();
  await openGameMenu(page);
  await button.click();
  await expect(
    page.getByRole("heading", { name: "Create room" }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Create room" }),
  ).toBeVisible();
  await page.screenshot({ path: "test-results/localization-en-tablet.png" });
});
