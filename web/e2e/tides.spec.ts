import { test, expect, type APIRequestContext } from "@playwright/test";
import { openGameMenu } from "./helpers/game-menu";
import { createHash } from "node:crypto";
import sharp from "sharp";
import { TidesCardID } from "../src/enums";
import { tidesArt, tidesArtURL } from "../src/cards/tides-art";

test("all six HD Tides images match their source revisions and dimensions", async ({
  request,
}) => {
  for (const id of Object.values(TidesCardID)) {
    const response = await request.get(tidesArtURL(id));
    test.skip(response.status() === 404, "Local artwork is optional in CI");
    expect(response.ok()).toBe(true);
    expect(response.headers()["content-type"]).toContain("image/webp");
    const body = await response.body();
    expect(createHash("sha256").update(body).digest("hex").slice(0, 12)).toBe(
      tidesArt[id].revision,
    );
    const decoded = await sharp(body).metadata();
    expect(decoded.format).toBe("webp");
    expect(decoded.width).toBe(tidesArt[id].width);
    expect(decoded.height).toBe(tidesArt[id].height);
  }
});

test("host can enable/disable Tides before start, but cannot change the mode during play", async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "settings-only test" }),
  );
  await page.goto("/");
  const toggle = page.getByRole("checkbox", { name: /Карты перевеса/ });
  await expect(toggle).not.toBeChecked();
  await toggle.check();
  await page.getByLabel("Имя игрока", { exact: true }).fill("Tides setup");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(
    page.getByRole("button", { name: "Начать партию" }),
  ).toBeVisible();
  const id = new URL(page.url()).searchParams.get("room");
  const path = `/api/rooms/${id}`;
  await expect
    .poll(
      async () =>
        (await page.request.get(path).then((r) => r.json())).settings
          .tidesOfBattle,
    )
    .toBe(true);
  await openGameMenu(page);
  await page
    .getByRole("button", { name: "Настройки комнаты", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("checkbox", { name: /Карты перевеса/ })
    .uncheck();
  await page.getByRole("button", { name: "Сохранить настройки" }).click();
  await expect
    .poll(
      async () =>
        (await page.request.get(path).then((r) => r.json())).settings
          .tidesOfBattle,
    )
    .toBe(false);
  await page.getByRole("button", { name: "Начать партию" }).click();
  await openGameMenu(page);
  await page
    .getByRole("button", { name: "Настройки комнаты", exact: true })
    .click();
  await expect(
    page.getByRole("dialog").getByRole("checkbox", { name: /Карты перевеса/ }),
  ).toBeDisabled();
  const state = await page.request.get(path).then((r) => r.json());
  const response = await page.request.post(path + "/commands", {
    data: {
      type: "settings",
      revision: state.revision,
      settings: { ...state.settings, tidesOfBattle: true },
    },
  });
  expect(response.status()).toBe(400);
  expect(
    (await page.request.get(path).then((r) => r.json())).settings.tidesOfBattle,
  ).toBe(false);
});

test("real combat persists private draws, reveals both cards, applies Blade and displays original images", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(90000);
  const contexts = [];
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "combat-only test" }),
  );
  const created = await page.request
    .post("/api/rooms", {
      data: {
        name: "Tides host",
        settings: {
          turnMinutes: 5,
          hardTimer: false,
          scenario: "base",
          tidesOfBattle: true,
        },
      },
    })
    .then((r) => r.json());
  const path = `/api/rooms/${created.id}`;
  const snapshot = (client = page.request) =>
    client.get(path).then((r) => r.json());
  const command = async (
    client: APIRequestContext,
    data: Record<string, unknown>,
  ) => {
    const state = await snapshot(client);
    const response = await client.post(path + "/commands", {
      data: { ...data, revision: state.revision },
    });
    expect(response.ok(), await response.text()).toBe(true);
    return response.json();
  };
  const guests = new Map<string, APIRequestContext>();
  try {
    for (const seat of created.seats.filter(
      (seat: { house: string }) => seat.house !== created.yourHouse,
    )) {
      await command(page.request, {
        type: "configure",
        house: seat.house,
        bot: false,
        difficulty: "normal",
      });
      const context = await browser.newContext({ baseURL });
      contexts.push(context);
      const joined = await context.request.post(path + "/join", {
        data: { name: seat.house, house: seat.house },
      });
      expect(joined.ok()).toBe(true);
      guests.set(seat.house, context.request);
    }
    await command(page.request, { type: "start" });
    const greyjoy = guests.get("greyjoy")!;
    await command(page.request, {
      type: "order",
      region: "winterfell",
      order: "march",
    });
    // Keep one more march pending so the round does not immediately finish
    // (and correctly refresh the Blade) as soon as this combat ends.
    await command(page.request, {
      type: "order",
      region: "white-harbor",
      order: "march-minus",
    });
    await command(greyjoy, {
      type: "order",
      region: "greywater-watch",
      order: "march",
    });
    for (const client of guests.values())
      await command(client, { type: "ready" });
    await command(page.request, { type: "ready" });
    await command(page.request, { type: "continue" });
    const raven = (await snapshot()).raven;
    await command(
      raven.house === created.yourHouse
        ? page.request
        : guests.get(raven.house)!,
      { type: "raven", choice: "skip" },
    );
    await command(page.request, {
      type: "march",
      region: "winterfell",
      target: "moat-cailin",
      units: { footman: 1, knight: 0 },
    });
    await command(greyjoy, {
      type: "march",
      region: "greywater-watch",
      target: "moat-cailin",
      units: { footman: 1 },
    });
    await command(page.request, { type: "card", card: "eddard-stark" });
    await command(greyjoy, { type: "card", card: "dagmar-cleftjaw" });
    const state = await snapshot();
    expect(state.battle.choice.kind).toBe("tides_redraw");
    expect(Object.keys(state.battle.modifiers)).toEqual(["stark"]);
    const greyState = await snapshot(greyjoy);
    expect(Object.keys(greyState.battle.modifiers)).toEqual(["greyjoy"]);
    expect(greyState.battle.modifiersRevealed).toBe(false);
    expect(JSON.stringify(greyState)).not.toContain('"deck"');
    const context = contexts.find((context) => context.request === greyjoy)!;
    const opponent = await context.newPage();
    opponent.on("pageerror", (error) => errors.push(error.message));
    await opponent.addInitScript(() =>
      localStorage.setItem("westeros-tour-v2", "true"),
    );
    await opponent.route(
      "**/local-assets/preview3d/game-manifest.json",
      (route) => route.fulfill({ status: 404, body: "combat-only test" }),
    );
    await opponent.goto(`/?room=${created.id}`);
    await opponent
      .locator(".combat-intro")
      .getByRole("button", { name: "Пропустить движение", exact: true })
      .click({ timeout: 20000 });
    const dialog = opponent.getByTestId("card-effect-choice");
    await expect(dialog).toContainText("Карты перевеса");
    await expect(dialog.locator('[data-card-id="hidden"]')).toHaveCount(1);
    await opponent.reload();
    await expect(dialog).toBeVisible();
    expect((await snapshot(greyjoy)).battle.modifiers.greyjoy).toEqual(
      greyState.battle.modifiers.greyjoy,
    );
    await dialog
      .getByRole("button", { name: "Оставить без применения меча" })
      .click();
    await expect(dialog.locator('[data-card-id="hidden"]')).toHaveCount(0);
    await expect(dialog.getByTestId("tides-card")).toHaveCount(2);
    await dialog.getByTestId("choice-language-switch").click();
    await expect(dialog).toContainText("Tides of Battle");
    await dialog.getByRole("button", { name: "Add +1 · spend Blade" }).click();
    await expect(dialog).toHaveCount(0);
    let after = await snapshot(greyjoy);
    const result = after.match.lastCombat;
    expect(after.round).toBe(state.round);
    expect(after.match.bladeUsed).toBe(true);
    expect(result.bladeBonus).toBe("greyjoy");
    expect(Object.keys(result.modifiers).sort()).toEqual(["greyjoy", "stark"]);
    expect(result.attack).toBe(3 + result.modifiers.greyjoy.strength);
    expect(result.defense).toBe(5 + result.modifiers.stark.strength);
    if (after.phase === "retreat") {
      const chooser = after.battle.retreatChooser || after.battle.loser;
      const client = chooser === "stark" ? page.request : guests.get(chooser)!;
      const view = await snapshot(client);
      await command(client, {
        type: "retreat",
        target: view.battle.retreats[0],
      });
    }
    await expect(opponent.locator(".combat-result-cards")).toBeVisible();
    await opponent
      .locator(".combat-report")
      .getByRole("button", { name: "Show result", exact: true })
      .click();
    await expect(opponent.locator(".combat-victory-banner")).toHaveCount(1);
    await expect(opponent.locator(".combat-defeat-slash")).toHaveCount(1);
    await expect(
      opponent.locator(".combat-card-tableau .house-card"),
    ).toHaveCount(2);
    await expect(
      opponent.locator(".combat-house-plaque .strength-shield > span").nth(0),
    ).toHaveText(String(result.attack));
    await expect(
      opponent.locator(".combat-house-plaque .strength-shield > span").nth(1),
    ).toHaveText(String(result.defense));
    const cards = opponent
      .locator(".combat-result-cards")
      .getByTestId("tides-card");
    await expect(cards).toHaveCount(2);
    for (const face of await cards.all()) {
      await expect
        .poll(() =>
          face.evaluate((element) => {
            const image = element.querySelector("img");
            return image
              ? image.complete && image.naturalWidth > 0
              : !!element.querySelector(".tides-card-fallback");
          }),
        )
        .toBe(true);
      const image = face.locator("img");
      if (await image.count()) {
        const ratio = await image.evaluate((img) => {
          const bounds = img.getBoundingClientRect();
          return Math.abs(
            bounds.height / bounds.width - img.naturalHeight / img.naturalWidth,
          );
        });
        expect(ratio).toBeLessThan(0.02);
        await expect(image).toHaveAttribute("src", /\.webp\?v=[a-f0-9]{12}$/);
      }
    }
    await expect(opponent.locator(".combat-result-cards")).toContainText(
      "Valyrian Steel Blade: +1 strength",
    );
    await opponent.screenshot({
      path: "test-results/tides-result.png",
      animations: "disabled",
    });
    await opponent.setViewportSize({ width: 390, height: 844 });
    await expect(opponent.locator(".combat-report")).toBeInViewport();
    expect(
      await opponent.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await opponent.locator(".combat-continue").scrollIntoViewIfNeeded();
    await expect(opponent.locator(".combat-continue")).toBeInViewport();
    await opponent.screenshot({
      path: "test-results/strategy-combat-mobile.png",
      animations: "disabled",
    });
    await opponent.keyboard.press("Escape");
    await expect(opponent.locator(".combat-report")).toHaveCount(0);
    expect(errors).toEqual([]);
  } finally {
    await Promise.all(contexts.map((context) => context.close()));
  }
});
