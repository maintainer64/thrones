import { test, expect } from "@playwright/test";
import type { APIRequestContext, Page } from "@playwright/test";
import type { Room } from "../src/types";

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_EVENTS_FIXTURES || "{}",
);
test.skip(
  !process.env.WESTEROS_EVENTS_FIXTURES,
  "Run TestEventsBrowser with real Go HTTP/WS and temporary SQLite",
);

async function open(
  page: Page,
  fixture: Fixture,
  baseURL: string,
  house = "stark",
) {
  const cookie = fixture.cookies[house];
  const probe = await page.request.get(`${baseURL}/api/rooms/${fixture.id}`, {
    headers: { Cookie: `${cookie.Name}=${cookie.Value}` },
  });
  expect(probe.status(), await probe.text()).toBe(200);
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL }]);
  await page.addInitScript(() => {
    localStorage.setItem("westeros-tour-v2", "true");
    localStorage.setItem("westeros-language", "ru");
  });
  await page.goto(`${baseURL}/?room=${fixture.id}`);
  await expect(page.getByRole("dialog")).toBeVisible({ timeout: 20000 });
}
async function state(
  api: APIRequestContext,
  fixture: Fixture,
  house = "stark",
): Promise<Room> {
  const cookie = fixture.cookies[house];
  const response = await api.get(`/api/rooms/${fixture.id}`, {
    headers: { Cookie: `${cookie.Name}=${cookie.Value}` },
  });
  expect(response.ok()).toBe(true);
  return response.json();
}
async function command(
  api: APIRequestContext,
  fixture: Fixture,
  house: string,
  value: Record<string, unknown>,
) {
  const current = await state(api, fixture, house);
  const cookie = fixture.cookies[house];
  const response = await api.post(`/api/rooms/${fixture.id}/commands`, {
    headers: { Cookie: `${cookie.Name}=${cookie.Value}` },
    data: { ...value, revision: current.revision },
  });
  expect(response.ok(), await response.text()).toBe(true);
}
async function choose(page: Page, name: string) {
  const modal = page.getByRole("dialog");
  await modal.getByRole("button", { name, exact: true }).click();
  await modal
    .getByRole("button", { name: "Подтвердить выбор", exact: true })
    .click();
}

test("secret contribution confirms once, survives reload, translates and never reveals another house’s bid", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(120000);
  const fixture = fixtures.privacy;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await expect(
    modal.getByRole("heading", { name: "Защита Стены", exact: true }),
  ).toBeVisible();
  await expect(
    modal.getByRole("heading", {
      name: "Карта одичалых пока скрыта",
      exact: true,
    }),
  ).toBeVisible();
  await modal.getByLabel("Ваш взнос", { exact: true }).fill("5");
  await modal
    .getByRole("button", { name: "Продолжить со взносом", exact: true })
    .click();
  expect(
    (await state(page.request, fixture)).wildlings!.attack!.bids.yourBid,
  ).toBeUndefined();
  await expect(
    modal.getByRole("button", { name: "Подтвердить взнос", exact: true }),
  ).toBeFocused();
  await modal
    .getByRole("button", { name: "Подтвердить взнос", exact: true })
    .click();
  await expect(modal.getByTestId("watch-your-bid")).toHaveText("5");
  await expect(modal.getByTestId("watch-power")).toHaveText("20");
  await page.reload();
  await expect(modal.getByTestId("watch-your-bid")).toHaveText("5");
  const guestContext = await browser.newContext();
  try {
    const guest = await guestContext.newPage();
    await open(guest, fixture, baseURL!, "greyjoy");
    await expect(
      guest.getByRole("heading", { name: "Готовы: 1 / 6", exact: true }),
    ).toBeVisible();
    await expect(guest.getByTestId("watch-your-bid")).toHaveCount(0);
    await expect(guest.getByTestId("watch-bid-stark")).toHaveText("Готов");
    const foreign = await state(guest.request, fixture, "greyjoy");
    expect(foreign.wildlings!.attack!.bids.values).toBeUndefined();
    expect(foreign.wildlings!.attack!.card).toBeUndefined();
    await modal
      .getByRole("button", { name: "Сменить язык", exact: true })
      .click();
    await expect(
      modal.getByRole("heading", { name: "Defend the Wall", exact: true }),
    ).toBeVisible();
    await expect(modal.getByTestId("watch-your-bid")).toHaveText("5");
    await page.keyboard.press("Escape");
    await expect(modal).toHaveCount(0);
    await page
      .getByRole("button", { name: "Events and Night’s Watch", exact: true })
      .click();
    await expect(modal.getByTestId("watch-your-bid")).toHaveText("5");
    await page.screenshot({
      path: test.info().outputPath("watch-secret-en.png"),
    });
  } finally {
    await guestContext.close();
  }
  expect(errors).toEqual([]);
});

test("zero contribution and all-zero tie work on a tablet, then resume planning with visible history", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.zero;
  await page.setViewportSize({ width: 820, height: 1180 });
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await modal
    .getByRole("button", { name: "Продолжить со взносом", exact: true })
    .click();
  await modal
    .getByRole("button", { name: "Подтвердить взнос", exact: true })
    .click();
  await expect(
    modal.getByRole("heading", {
      name: "Решение Железного трона",
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    modal.getByText("Карта одичалых пока скрыта", { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: test.info().outputPath("watch-tablet-tie.png"),
  });
  await choose(page, "Выбрать дом Грейджой");
  await expect
    .poll(async () => (await state(page.request, fixture)).phase, {
      timeout: 20000,
    })
    .toBe("planning");
  await modal.getByRole("button", { name: "Продолжить", exact: true }).click();
  await expect(
    modal.getByRole("heading", { name: "События Вестероса", exact: true }),
  ).toBeVisible();
  await modal.locator(".events-last summary").click();
  await expect(modal.locator(".events-last")).toContainText("Тишина у Стены");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
  ).toBe(false);
});

test("unit losses require confirmation, map inspection and reload preserve the same-castle restriction", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.units;
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await expect(
    modal.getByRole("heading", {
      name: "Выберите отряд для уничтожения",
      exact: true,
    }),
  ).toBeVisible();
  await modal
    .getByRole("button", {
      name: "Показать область на карте: Винтерфелл — Пехота",
      exact: true,
    })
    .click();
  await expect(modal).toHaveCount(0);
  await expect(page.locator(".inspector h2")).toHaveText("Винтерфелл");
  await page
    .getByRole("button", { name: "События и Ночной дозор", exact: true })
    .click();
  await modal
    .getByRole("button", { name: "Винтерфелл — Пехота", exact: true })
    .click();
  expect(
    (await state(page.request, fixture)).match!.armies.winterfell.reduce(
      (sum, unit) => sum + unit.quantity,
      0,
    ),
  ).toBe(2);
  await modal
    .getByRole("button", { name: "Подтвердить выбор", exact: true })
    .click();
  await expect(
    modal.getByText("Осталось выбрать: 1", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    modal.getByRole("button", { name: "Белая Гавань — Пехота", exact: true }),
  ).toHaveCount(0);
  await choose(page, "Винтерфелл — Рыцари");
  await expect
    .poll(async () => (await state(page.request, fixture)).wildlings?.attack, {
      timeout: 15000,
    })
    .toBeUndefined();
  const after = await state(page.request, fixture);
  expect(after.match!.armies.winterfell || []).toHaveLength(0);
  expect(after.match!.armies["white-harbor"][0].quantity).toBe(2);
});

test("a discarded house card can be retrieved through the real card UI", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.cards;
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await expect(
    modal.getByRole("heading", {
      name: "Верните одну карту дома из сброса",
      exact: true,
    }),
  ).toBeVisible();
  await expect(modal.locator(".events-card-option .house-card")).toHaveCount(2);
  await choose(page, "Эддард Старк");
  await expect
    .poll(async () => (await state(page.request, fixture)).match!.discard.stark)
    .toEqual(["robb-stark"]);
});

test("Preemptive Raid delegates tied highest tracks to the current throne holder", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(120000);
  const fixture = fixtures.tracks;
  await open(page, fixture, baseURL!);
  await choose(page, "Снизить влияние на высшем треке");
  await expect(
    page
      .getByRole("dialog")
      .getByText("Решение принимает дом Грейджой", { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("dialog")
      .getByRole("button", { name: "Вотчины", exact: true }),
  ).toBeDisabled();
  const context = await browser.newContext();
  try {
    const ruler = await context.newPage();
    await open(ruler, fixture, baseURL!, "greyjoy");
    await ruler.bringToFront();
    await expect(
      ruler
        .getByRole("dialog")
        .getByRole("button", { name: "Вотчины", exact: true }),
    ).toBeEnabled();
    await choose(ruler, "Вотчины");
    await expect
      .poll(async () =>
        (await state(page.request, fixture)).match!.tracks.fiefdoms.indexOf(
          "stark",
        ),
      )
      .toBe(3);
  } finally {
    await context.close();
  }
});

test("Horde reward enters the existing single-castle mustering dialog and returns to events", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(90000);
  const fixture = fixtures.castle;
  await open(page, fixture, baseURL!);
  await choose(page, "Винтерфелл");
  // The selected castle is persisted first; the server timer enters the
  // shared mustering resolver on its next tick, independently of the browser.
  await expect
    .poll(async () => (await state(page.request, fixture)).phase, {
      timeout: 15000,
    })
    .toBe("mustering");
  const modal = page.getByRole("dialog");
  // Bot/timer updates may not replace the card a player is still reading.
  await expect(modal).toHaveCount(1);
  await expect(
    modal.getByRole("heading", { name: "Сбор войск", exact: true }),
  ).toHaveCount(0);
  await expect(
    modal.locator(".events-options button:not(:disabled)"),
  ).toHaveCount(0);
  await modal.getByRole("button", { name: "Продолжить", exact: true }).click();
  await expect(
    modal.getByRole("heading", { name: "Сбор войск", exact: true }),
  ).toBeVisible({ timeout: 20000 });
  await expect(
    modal
      .getByRole("navigation", { name: "Области решения" })
      .getByRole("button"),
  ).toHaveCount(1);
  await expect(modal.locator(".muster-current-units")).toContainText("Пехота");
  await modal.getByRole("button", { name: /Улучшить пехоту → Рыцари/ }).click();
  await modal
    .getByRole("button", { name: "Завершить сбор войск", exact: true })
    .click();
  await modal
    .getByRole("button", {
      name: "Завершить и потерять остаток очков",
      exact: true,
    })
    .click();
  await expect
    .poll(async () => (await state(page.request, fixture)).phase, {
      timeout: 15000,
    })
    .toBe("planning");
});

test("Westeros dominance choice uses server options and history without untranslated IDs", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.event;
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await expect(modal.locator(".events-westeros article")).toHaveCount(3);
  await choose(page, "Без дополнительного эффекта");
  await expect
    .poll(async () => (await state(page.request, fixture)).phase, {
      timeout: 15000,
    })
    .toBe("planning");
  await modal
    .getByRole("button", { name: "Сменить язык", exact: true })
    .click();
  await modal.locator(".events-history summary").click();
  await expect(modal.locator(".events-history")).not.toContainText(
    "a-throne-of-blades",
  );
  await expect(modal.locator(".events-westeros")).toContainText(
    "A Throne of Blades",
  );
  await page.screenshot({
    path: test.info().outputPath("westeros-events-en.png"),
  });
});

test("repeat attack shows exemption and last public card while the next card remains hidden", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.repeat;
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await expect(modal.locator(".events-excluded")).toContainText("Старк");
  await expect(modal.getByLabel("Ваш взнос", { exact: true })).toHaveCount(0);
  await expect(
    modal.getByRole("heading", { name: "Готовы: 0 / 5", exact: true }),
  ).toBeVisible();
  await expect(modal.locator(".events-last summary")).toContainText(
    "Передовой отряд",
  );
  expect(
    (await state(page.request, fixture)).wildlings!.attack!.card,
  ).toBeUndefined();
  for (const house of [
    "greyjoy",
    "lannister",
    "baratheon",
    "tyrell",
    "martell",
  ]) {
    await command(page.request, fixture, house, {
      type: "wildling_bid",
      bid: 0,
    });
  }
  await choose(page, "Выбрать дом Грейджой");
  await expect
    .poll(
      async () => (await state(page.request, fixture)).wildlings!.last!.card,
    )
    .toBe("silence-at-the-wall");
});
