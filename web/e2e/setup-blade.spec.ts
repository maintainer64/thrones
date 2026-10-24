import { test, expect, type Page } from "@playwright/test";
import type { Room } from "../src/types";
import { openGameMenu } from "./helpers/game-menu";
import {
  captureVisualCheckpoint,
  throttleBrowserCPU,
  waitForRoomRevision,
} from "./helpers/runtime";

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_SETUP_BLADE_FIXTURES || "{}",
);
test.beforeEach(async ({ page }) => {
  await throttleBrowserCPU(page);
  await page.addInitScript(() => {
    localStorage.setItem("westeros-tour-v2", "true");
    localStorage.setItem("westeros-language", "ru");
    localStorage.setItem("westeros-motion", "false");
  });
});

for (const count of [3, 4, 5, 6]) {
  test(`base setup for ${count} houses previews the actual armies and fortifications`, async ({
    page,
  }) => {
    test.setTimeout(90000);
    if (count === 3) await page.setViewportSize({ width: 820, height: 1180 });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto("/");
    await page
      .getByLabel("Число играющих домов", { exact: true })
      .selectOption(String(count));
    await expect(page.getByRole("radio")).toHaveCount(count);
    if (count < 6)
      await expect(
        page.getByRole("radio", { name: "Мартелл", exact: true }),
      ).toHaveCount(0);
    await page.getByLabel("Имя игрока", { exact: true }).fill(`Setup ${count}`);
    await page
      .getByRole("button", { name: "Собрать малый совет", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Начать партию", exact: true }),
    ).toBeVisible();
    const id = new URL(page.url()).searchParams.get("room");
    const snapshot = async (): Promise<Room> =>
      page.request.get(`/api/rooms/${id}`).then((r) => r.json());
    const before = await snapshot();
    expect(before.seats).toHaveLength(count);
    expect(before.setup?.playerCount).toBe(count);
    const pieces = Object.values(before.setup!.armies)
      .flat()
      .reduce((total, u) => total + u.quantity, 0);
    const board = page.getByTestId("game3d-board");
    await expect(board).toHaveAttribute("data-ready", "true", {
      timeout: 60000,
    });
    await expect(board).toHaveAttribute("data-piece-count", String(pieces));
    await expect(
      page.locator(
        '[data-region3d="winterfell"] [data-fortification="garrison"]',
      ),
    ).toContainText("2");
    await expect(
      page.locator(
        '[data-region3d="kings-landing"] [data-fortification="neutral"]',
      ),
    ).toContainText("5");
    await expect(
      page.getByRole("region", { name: "Гарнизон дома", exact: true }),
    ).toContainText("После поражения удаляется навсегда");
    await expect(page.locator('[data-fortification="blocked"]')).toHaveCount(
      count === 3 ? 16 : 0,
    );
    await page
      .getByRole("button", { name: "Начать партию", exact: true })
      .click();
    await expect.poll(async () => (await snapshot()).phase).toBe("planning");
    const playing = await snapshot();
    expect(playing.match!.armies).toEqual(before.setup!.armies);
    expect(playing.match!.tracks).toEqual(before.setup!.tracks);
    expect(playing.orderBudget!.starsAllowed).toBe(count <= 4 ? 2 : 3);
    await expect(page.locator(".order-budget")).toContainText(
      `Особые приказы: осталось ${playing.orderBudget!.starsAllowed}`,
    );
    await page.reload();
    await expect(board).toHaveAttribute("data-ready", "true", {
      timeout: 60000,
    });
    await expect(board).toHaveAttribute("data-piece-count", String(pieces));
    await expect(
      page.locator(
        '[data-region3d="winterfell"] [data-fortification="garrison"]',
      ),
    ).toBeVisible();
    await captureVisualCheckpoint(
      page,
      test.info().outputPath(`setup-${count}-board.png`),
    );
    await openGameMenu(page);
    await page
      .getByRole("button", { name: "Настройки комнаты", exact: true })
      .click();
    await expect(
      page
        .getByRole("dialog")
        .getByLabel("Число играющих домов", { exact: true }),
    ).toBeDisabled();
    await captureVisualCheckpoint(
      page,
      test.info().outputPath(`setup-${count}.png`),
    );
    expect(errors).toEqual([]);
  });
}

async function state(page: Page, fixture: Fixture): Promise<Room> {
  return page.request.get(`/api/rooms/${fixture.id}`).then((r) => r.json());
}
async function command(
  page: Page,
  fixture: Fixture,
  house: string,
  body: Record<string, unknown>,
) {
  const cookie = fixture.cookies[house];
  const headers = { Cookie: `${cookie.Name}=${cookie.Value}` };
  const current: Room = await page.request
    .get(`/api/rooms/${fixture.id}`, { headers })
    .then((r) => r.json());
  const response = await page.request.post(
    `/api/rooms/${fixture.id}/commands`,
    { headers, data: { ...body, revision: current.revision } },
  );
  expect(response.ok(), await response.text()).toBe(true);
  const next: Room = await response.json();
  await waitForRoomRevision(page, fixture.id, next.revision);
}

for (const mode of ["use", "keep", "used"]) {
  test(`ordinary Blade ${mode}: real empty-garrison combat, persistent decision and correct strength`, async ({
    page,
    baseURL,
  }) => {
    test.skip(
      !process.env.WESTEROS_SETUP_BLADE_FIXTURES,
      "Run TestSetupBladeBrowser with real Go HTTP/WS/SQLite",
    );
    test.setTimeout(90000);
    const fixture = fixtures[mode];
    const cookie = fixture.cookies.stark;
    await page
      .context()
      .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL! }]);
    await page.goto(`/?room=${fixture.id}`);
    await waitForRoomRevision(
      page,
      fixture.id,
      (await state(page, fixture)).revision,
    );
    await page
      .locator(".combat-intro")
      .getByRole("button", { name: "Пропустить движение", exact: true })
      .click();
    const modal = page.getByRole("dialog");
    await modal
      .locator(".battle-hand button")
      .filter({ has: page.locator("span", { hasText: "Кейтилин Старк" }) })
      .click();
    await expect
      .poll(async () => (await state(page, fixture)).battle?.yourCard, {
        timeout: 20000,
        message:
          "The player's house card is committed before the opponent chooses",
      })
      .toBe("catelyn-stark");
    await command(page, fixture, "greyjoy", {
      type: "card",
      card: "dagmar-cleftjaw",
    });
    if (mode !== "used") {
      await expect(page.getByTestId("card-effect-choice")).toBeVisible({
        timeout: 20000,
      });
      await expect(
        modal.getByRole("heading", {
          name: "Валирийский меч · ваше решение",
          exact: true,
        }),
      ).toBeVisible();
      await expect(modal.locator(".tides-pair")).toHaveCount(0);
      expect((await state(page, fixture)).match!.lastCombat).toBeUndefined();
      await page.reload();
      await waitForRoomRevision(
        page,
        fixture.id,
        (await state(page, fixture)).revision,
      );
      await expect(page.getByTestId("card-effect-choice")).toBeVisible({
        timeout: 20000,
      });
      await page.getByTestId("choice-language-switch").click();
      await expect(
        modal.getByRole("heading", {
          name: "Valyrian Steel Blade · your decision",
          exact: true,
        }),
      ).toBeVisible();
      await modal
        .getByRole("button", {
          name:
            mode === "use"
              ? "Add +1 · spend Blade"
              : "Keep without using Blade",
          exact: true,
        })
        .click();
    }
    await expect
      .poll(async () => (await state(page, fixture)).match?.lastCombat?.winner)
      .toBe(mode === "use" ? "stark" : "greyjoy");
    const result = await state(page, fixture);
    expect(result.match!.lastCombat!.attack).toBe(3);
    expect(result.match!.lastCombat!.defense).toBe(mode === "use" ? 3 : 2);
    expect(result.match!.bladeUsed).toBe(mode !== "keep");
    expect(result.match!.lastCombat!.modifiers).toBeUndefined();
    expect(result.match!.lastCombat!.bladeBonus).toBe(
      mode === "use" ? "stark" : undefined,
    );
    if (mode === "use") {
      await command(page, fixture, "greyjoy", {
        type: "retreat",
        target: "white-harbor",
      });
    } else {
      expect(result.fortifications?.winterfell).toBeUndefined();
    }
    await expect(
      page.getByRole("dialog").locator(".combat-result-cards"),
    ).toBeVisible();
    await captureVisualCheckpoint(
      page,
      test.info().outputPath(`blade-${mode}.png`),
    );
  });
}
