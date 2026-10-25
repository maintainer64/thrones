import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "lobby-only test" }),
  );
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
});

test("host and invited players choose their houses, restore seats and cannot steal a house", async ({
  page,
  browser,
  baseURL,
}, testInfo) => {
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("Host Martell");
  await page
    .locator(".house-picker-option")
    .filter({ hasText: "Мартелл" })
    .click();
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(page.locator(".my-house h2")).toHaveText("Мартелл");
  await page
    .getByRole("button", { name: "Панель великих домов", exact: true })
    .click();
  const roomURL = page.url();
  const id = new URL(roomURL).searchParams.get("room");
  const state = () =>
    page.request.get(`/api/rooms/${id}`).then((r) => r.json());
  await expect.poll(async () => (await state()).yourHouse).toBe("martell");
  const context = await browser.newContext({ baseURL });
  try {
    const guest = await context.newPage();
    await guest.addInitScript(() =>
      localStorage.setItem("westeros-tour-v2", "true"),
    );
    await guest.route("**/local-assets/preview3d/game-manifest.json", (route) =>
      route.fulfill({ status: 404, body: "lobby-only test" }),
    );
    await guest.goto(roomURL);
    await guest.getByLabel("Имя игрока", { exact: true }).fill("Guest Stark");
    await expect(guest.getByRole("radio", { name: /Мартелл/ })).toBeDisabled();
    await expect(
      guest.getByRole("radio", { name: "Старк", exact: true }),
    ).toBeEnabled();
    await guest
      .locator(".house-picker-option")
      .filter({ hasText: "Старк" })
      .click();
    await guest.screenshot({
      path: testInfo.outputPath("invitation-choice.png"),
    });
    await guest
      .getByRole("button", { name: "Присоединиться", exact: true })
      .click();
    await expect(guest.locator(".my-house h2")).toHaveText("Старк");
    await guest
      .getByRole("button", { name: "Панель великих домов", exact: true })
      .click();
    await expect(page.getByRole("radio", { name: /Старк/ })).toBeDisabled();
    await guest
      .locator(".house-picker-option")
      .filter({ hasText: "Грейджой" })
      .click();
    await expect(guest.locator(".my-house h2")).toHaveText("Грейджой");
    await expect(
      page.getByRole("radio", { name: "Старк", exact: true }),
    ).toBeEnabled();
    await page
      .locator(".house-picker-option")
      .filter({ hasText: "Старк" })
      .click();
    await expect(page.locator(".my-house h2")).toHaveText("Старк");
    const before = await state();
    const forbidden = await page.request.post(`/api/rooms/${id}/commands`, {
      data: {
        type: "choose_house",
        house: "greyjoy",
        revision: before.revision,
      },
    });
    expect(forbidden.status()).toBe(400);
    expect((await state()).yourHouse).toBe("stark");
    await page.reload();
    await guest.reload();
    await expect(page.locator(".my-house h2")).toHaveText("Старк");
    await expect(guest.locator(".my-house h2")).toHaveText("Грейджой");
    await expect(
      guest.getByRole("button", { name: "Начать партию", exact: true }),
    ).toHaveCount(0);
    await page.screenshot({ path: testInfo.outputPath("lobby-choice.png") });
    await page
      .getByRole("button", { name: "Начать партию", exact: true })
      .click();
    await expect.poll(async () => (await state()).phase).toBe("planning");
    await expect(page.locator(".house-picker")).toHaveCount(0);
    await expect(guest.locator(".house-picker")).toHaveCount(0);
    const started = await state();
    expect(started.seats.filter((s: { bot: boolean }) => s.bot)).toHaveLength(
      4,
    );
    const late = await page.request.post(`/api/rooms/${id}/commands`, {
      data: {
        type: "choose_house",
        house: "martell",
        revision: started.revision,
      },
    });
    expect(late.status()).toBe(400);
  } finally {
    await context.close();
  }
});

test("solo player chooses Lannister on a tablet and starts with five bots", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto("/?lang=en");
  await page.getByLabel("Player name", { exact: true }).fill("Solo Lannister");
  await page
    .locator(".house-picker-option")
    .filter({ hasText: "Lannister" })
    .click();
  await page.screenshot({
    path: testInfo.outputPath("tablet-house-choice.png"),
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("button", { name: "Gather the Small Council" }).click();
  await expect(page.locator(".my-house h2")).toHaveText("Lannister");
  await page.getByRole("button", { name: "Start game", exact: true }).click();
  const id = new URL(page.url()).searchParams.get("room");
  await expect
    .poll(
      async () =>
        (await page.request.get(`/api/rooms/${id}`).then((r) => r.json()))
          .phase,
    )
    .toBe("planning");
  const state = await page.request
    .get(`/api/rooms/${id}`)
    .then((r) => r.json());
  expect(state.yourHouse).toBe("lannister");
  expect(state.phase).toBe("planning");
  expect(state.seats.filter((s: { bot: boolean }) => s.bot)).toHaveLength(5);
});
