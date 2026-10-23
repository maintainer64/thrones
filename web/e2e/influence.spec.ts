import { test, expect } from "@playwright/test";
import type { APIRequestContext, BrowserContext, Page } from "@playwright/test";
import type { Room } from "../src/types";

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_INFLUENCE_FIXTURES || "{}",
);
test.skip(
  !process.env.WESTEROS_INFLUENCE_FIXTURES,
  "Run through TestInfluenceBrowser: real Go engine + temporary SQLite, no production fixture endpoint",
);

function cookie(fixture: Fixture, house: string) {
  const value = fixture.cookies[house];
  return `${value.Name}=${value.Value}`;
}

async function openRoom(
  page: Page,
  fixture: Fixture,
  baseURL: string,
  house = "stark",
) {
  const value = fixture.cookies[house];
  await page
    .context()
    .addCookies([{ name: value.Name, value: value.Value, url: baseURL }]);
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
  const response = await api.get(`/api/rooms/${fixture.id}`, {
    headers: { Cookie: cookie(fixture, house) },
  });
  expect(response.ok()).toBe(true);
  return response.json();
}

async function command(
  api: APIRequestContext,
  fixture: Fixture,
  house: string,
  value: Record<string, unknown>,
): Promise<Room> {
  const current = await state(api, fixture, house);
  const response = await api.post(`/api/rooms/${fixture.id}/commands`, {
    headers: { Cookie: cookie(fixture, house) },
    data: { ...value, revision: current.revision },
  });
  expect(response.ok(), await response.text()).toBe(true);
  return response.json();
}

async function commit(page: Page, bid: number) {
  await page.bringToFront();
  const modal = page.getByRole("dialog");
  await modal.getByLabel("Ваша ставка", { exact: true }).fill(String(bid));
  await modal
    .getByRole("button", { name: "Продолжить со ставкой", exact: true })
    .click();
  await expect(
    modal.getByRole("heading", {
      name: `Подтвердить ставку: ${bid}?`,
      exact: true,
    }),
  ).toBeVisible();
  await modal
    .getByRole("button", { name: "Подтвердить ставку", exact: true })
    .click();
}

test("secret bid requires confirmation, survives reload and RU/EN without leaking to another player", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(120000);
  const fixture = fixtures.privacy;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await openRoom(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await modal.getByLabel("Ваша ставка", { exact: true }).fill("13");
  await modal
    .getByRole("button", { name: "Продолжить со ставкой", exact: true })
    .click();
  expect(
    (await state(page.request, fixture)).influence!.bids.yourBid,
  ).toBeUndefined();
  await modal
    .getByRole("button", { name: "Изменить сумму", exact: true })
    .click();
  await expect(modal.getByLabel("Ваша ставка", { exact: true })).toHaveValue(
    "13",
  );
  await commit(page, 13);
  await expect(modal.getByTestId("your-bid")).toHaveText("13");
  await expect(modal.getByTestId("available-power")).toHaveText("20");
  await expect(modal.getByLabel("Ваша ставка", { exact: true })).toHaveCount(0);
  const guestContext = await browser.newContext();
  try {
    const guest = await guestContext.newPage();
    await openRoom(guest, fixture, baseURL!, "greyjoy");
    await expect(
      guest.getByRole("heading", { name: "Готовы: 1 / 6", exact: true }),
    ).toBeVisible();
    await expect(guest.getByTestId("your-bid")).toHaveCount(0);
    await expect(guest.getByTestId("revealed-bid-stark")).toHaveCount(0);
    const foreign = await state(guest.request, fixture, "greyjoy");
    expect(foreign.influence!.bids.yourBid).toBeUndefined();
    expect(foreign.influence!.bids.values).toBeUndefined();
    expect(foreign.match!.power.stark).toBe(20);
    await page.bringToFront();
    await page.reload();
    await expect(modal.getByTestId("your-bid")).toHaveText("13");
    await modal
      .getByRole("button", { name: "Сменить язык", exact: true })
      .click();
    await expect(
      modal.getByRole("heading", { name: "Iron Throne", exact: true }),
    ).toBeVisible();
    await expect(modal.getByTestId("your-bid")).toHaveText("13");
    await page.keyboard.press("Escape");
    await expect(modal).toHaveCount(0);
    await page
      .getByRole("button", { name: "Open auction", exact: true })
      .click();
    await expect(modal.getByTestId("your-bid")).toHaveText("13");
    await page.screenshot({
      path: test.info().outputPath("influence-secret-en.png"),
    });
  } finally {
    await guestContext.close();
  }
  expect(errors).toEqual([]);
});

test("all three real auctions, old throne holder ties and final report work across browsers", async ({
  page,
  browser,
  baseURL,
}) => {
  // Three simultaneous 3D boards use the software GPU in CI. Allow cold model
  // loading, but still require each authoritative update to reach the real UI.
  test.setTimeout(180000);
  const fixture = fixtures.complete;
  const trace: unknown[] = [];
  const watch = (tab: Page, house: string) => {
    tab.on("pageerror", (error) => trace.push({ house, error: error.message }));
    tab.on("websocket", (socket) =>
      socket.on("framereceived", ({ payload }) => {
        try {
          const value = JSON.parse(String(payload));
          trace.push({
            house,
            revision: value.revision,
            phase: value.phase,
            track: value.influence?.track,
            stage: value.influence?.stage,
          });
        } catch {
          /* Only diagnostic JSON frames. */
        }
      }),
    );
  };
  watch(page, "stark");
  await openRoom(page, fixture, baseURL!);
  const contexts: BrowserContext[] = [];
  try {
    const rulers: Record<string, Page> = {};
    for (const house of ["baratheon", "greyjoy"]) {
      const context = await browser.newContext();
      contexts.push(context);
      rulers[house] = await context.newPage();
      watch(rulers[house], house);
      await openRoom(rulers[house], fixture, baseURL!, house);
    }
    const names: Record<string, string> = {
      stark: "Старк",
      greyjoy: "Грейджой",
      lannister: "Ланнистер",
      baratheon: "Баратеон",
      tyrell: "Тирелл",
      martell: "Мартелл",
    };
    const plans = [
      {
        track: "ironThrone",
        title: "Железный трон",
        ruler: "baratheon",
        picks: ["greyjoy", "lannister"],
        bids: {
          stark: 3,
          greyjoy: 3,
          baratheon: 2,
          lannister: 2,
          tyrell: 1,
          martell: 0,
        },
      },
      {
        track: "fiefdoms",
        title: "Вотчины",
        ruler: "greyjoy",
        picks: ["stark", "lannister"],
        bids: {
          stark: 3,
          greyjoy: 3,
          baratheon: 2,
          lannister: 2,
          tyrell: 1,
          martell: 0,
        },
      },
      {
        track: "kingsCourt",
        title: "Королевский двор",
        ruler: "greyjoy",
        picks: ["stark", "lannister"],
        bids: {
          stark: 2,
          greyjoy: 2,
          baratheon: 1,
          lannister: 1,
          tyrell: 3,
          martell: 0,
        },
      },
    ];
    for (const plan of plans) {
      await page.bringToFront();
      await expect(
        page
          .getByRole("dialog")
          .getByRole("heading", { name: plan.title, exact: true }),
      ).toBeVisible();
      await commit(page, plan.bids.stark);
      await expect(page.getByRole("dialog").getByTestId("your-bid")).toHaveText(
        String(plan.bids.stark),
      );
      for (const [house, bid] of Object.entries(plan.bids)) {
        if (house !== "stark")
          await command(page.request, fixture, house, { type: "bid", bid });
      }
      const ruler = rulers[plan.ruler];
      await ruler.bringToFront();
      // Activate the other player's window with an actual pointer event;
      // bringToFront alone need not focus an occluded Chrome window on macOS.
      await ruler.locator(".modal-top").click({ position: { x: 15, y: 15 } });
      await expect(
        ruler.getByRole("heading", { name: "Ставки раскрыты", exact: true }),
      ).toBeVisible({ timeout: 20000 });
      await page.bringToFront();
      await expect(
        page.getByRole("heading", {
          name: `Ожидаем решение дома ${names[plan.ruler]}`,
          exact: true,
        }),
      ).toBeVisible();
      expect((await state(page.request, fixture)).influence!.track).toBe(
        plan.track,
      );
      for (const house of plan.picks) {
        await ruler.bringToFront();
        await ruler
          .getByRole("button", {
            name: `Назначить дом ${names[house]}`,
            exact: true,
          })
          .click();
      }
      if (plan.track === "ironThrone") {
        await expect
          .poll(
            async () =>
              (await state(page.request, fixture)).match!.tracks.ironThrone[0],
          )
          .toBe("greyjoy");
      }
    }
    const modal = page.getByRole("dialog");
    await page.bringToFront();
    await expect(
      modal.getByRole("heading", {
        name: "Итоги торгов за влияние",
        exact: true,
      }),
    ).toBeVisible();
    const final = await state(page.request, fixture);
    expect(final.phase).toBe("westeros");
    expect(final.influence).toBeUndefined();
    expect(final.match!.lastInfluence).toHaveLength(3);
    expect(final.match!.tracks.ironThrone[0]).toBe("greyjoy");
    expect(final.match!.tracks.fiefdoms[0]).toBe("stark");
    expect(final.match!.tracks.kingsCourt[0]).toBe("tyrell");
    expect(final.match!.power.stark).toBe(12);
    await page.screenshot({
      path: test.info().outputPath("influence-results-ru.png"),
    });
    await modal
      .getByRole("button", { name: "Вернуться к полю", exact: true })
      .click();
    await page.reload();
    await page
      .getByRole("button", { name: "Итоги влияния", exact: true })
      .click();
    await expect(
      modal.getByRole("heading", {
        name: "Итоги торгов за влияние",
        exact: true,
      }),
    ).toBeVisible();
  } catch (error) {
    await test.info().attach("auction-websocket-trace.json", {
      body: JSON.stringify(trace, null, 2),
      contentType: "application/json",
    });
    for (const [index, context] of contexts.entries()) {
      for (const tab of context.pages()) {
        const diagnostics = await tab.evaluate(() => ({
          visible: document.visibilityState,
          headings: Array.from(
            document.querySelectorAll('[role="dialog"] h2, [role="dialog"] h3'),
          ).map((node) => node.textContent),
          boardRevision: document
            .querySelector("[data-revision]")
            ?.getAttribute("data-revision"),
        }));
        console.error("auction browser", index, JSON.stringify(diagnostics));
      }
    }
    console.error("auction frames", JSON.stringify(trace));
    throw error;
  } finally {
    for (const context of contexts) await context.close();
  }
});

test("tablet touch, keyboard focus, explicit zero and pending tie survive reload", async ({
  browser,
  baseURL,
}) => {
  test.setTimeout(90000);
  const fixture = fixtures.tablet;
  const context = await browser.newContext({
    viewport: { width: 768, height: 1024 },
    hasTouch: true,
  });
  try {
    const page = await context.newPage();
    await openRoom(page, fixture, baseURL!);
    const modal = page.getByRole("dialog");
    await expect(modal.getByTestId("available-power")).toHaveText("0");
    await expect(
      modal.getByRole("heading", { name: "Готовы: 5 / 6", exact: true }),
    ).toBeVisible();
    const first = modal.getByRole("button", {
      name: "Сменить язык",
      exact: true,
    });
    const last = modal.getByRole("button", {
      name: "Продолжить со ставкой",
      exact: true,
    });
    await first.focus();
    await page.keyboard.press("Shift+Tab");
    await expect(last).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(first).toBeFocused();
    await modal.getByLabel("Ваша ставка", { exact: true }).fill("1");
    await expect(last).toBeDisabled();
    await modal.getByRole("button", { name: "Поставить 0", exact: true }).tap();
    await last.tap();
    await modal
      .getByRole("button", { name: "Подтвердить ставку", exact: true })
      .tap();
    await expect(
      modal.getByRole("heading", {
        name: "Выберите дом для места №1",
        exact: true,
      }),
    ).toBeVisible();
    // Deterministically deliver the real WS update before its HTTP response.
    // New controls exist but stay disabled until the pending command finishes.
    let releaseResponse!: () => void;
    const responseGate = new Promise<void>((resolve) => {
      releaseResponse = resolve;
    });
    await page.route(
      `**/api/rooms/${fixture.id}/commands`,
      async (route) => {
        const response = await route.fetch();
        await responseGate;
        await route.fulfill({ response });
      },
      { times: 1 },
    );
    try {
      await modal
        .getByRole("button", { name: "Назначить дом Грейджой", exact: true })
        .tap();
      await expect(
        modal.getByRole("heading", {
          name: "Выберите дом для места №2",
          exact: true,
        }),
      ).toBeVisible();
      await expect(
        modal.getByRole("button", { name: "Назначить дом Старк", exact: true }),
      ).toBeDisabled();
    } finally {
      releaseResponse();
    }
    await expect(
      modal.getByRole("button", { name: "Назначить дом Старк", exact: true }),
    ).toBeFocused();
    await page.reload();
    await expect(
      modal.getByRole("heading", {
        name: "Выберите дом для места №2",
        exact: true,
      }),
    ).toBeVisible({ timeout: 20000 });
    expect((await state(page.request, fixture)).influence!.ranking).toEqual([
      "greyjoy",
    ]);
    await expect(
      modal.getByRole("button", {
        name: "Назначить дом Грейджой",
        exact: true,
      }),
    ).toHaveCount(0);
    const bounds = await modal.boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.y).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(768);
    expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(1024);
    await page.screenshot({
      path: test.info().outputPath("influence-tablet-tie.png"),
    });
  } finally {
    await context.close();
  }
});

test("a real concurrent revision conflict is visible and can be retried without duplicate bids", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.conflict;
  await openRoom(page, fixture, baseURL!);
  let conflict = true;
  await page.route("**/api/rooms/*/commands", async (route) => {
    if (conflict && route.request().postDataJSON()?.type === "bid") {
      conflict = false;
      await command(page.request, fixture, "greyjoy", { type: "bid", bid: 0 });
    }
    await route.continue();
  });
  await commit(page, 7);
  const modal = page.getByRole("dialog");
  await expect(modal.getByRole("alert")).toBeVisible();
  expect(
    (await state(page.request, fixture)).influence!.bids.yourBid,
  ).toBeUndefined();
  await modal
    .getByRole("button", { name: "Подтвердить ставку", exact: true })
    .click();
  await expect(modal.getByTestId("your-bid")).toHaveText("7");
  await expect(modal.getByRole("alert")).toHaveCount(0);
  const current = await state(page.request, fixture);
  expect(current.influence!.bids.submitted.stark).toBe(true);
  expect(current.influence!.bids.submitted.greyjoy).toBe(true);
  expect(
    Object.keys(current.influence!.bids.submitted).filter(
      (house) => current.influence!.bids.submitted[house],
    ),
  ).toHaveLength(2);
  expect(current.match!.power.stark).toBe(20);
});
