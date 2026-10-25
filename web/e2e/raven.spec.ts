import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import type { Room } from "../src/types";

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_RAVEN_FIXTURES || "{}",
);
test.skip(
  !process.env.WESTEROS_RAVEN_FIXTURES,
  "Run TestRavenBrowser: real Go HTTP/WS + SQLite, no production fixture API",
);

async function open(
  page: Page,
  fixture: Fixture,
  baseURL: string,
  house = "stark",
) {
  const cookie = fixture.cookies[house];
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL }]);
  await page.addInitScript(() => {
    localStorage.setItem("westeros-tour-v2", "true");
    localStorage.setItem("westeros-language", "ru");
  });
  await page.goto(`${baseURL}/?room=${fixture.id}`);
  await expect(
    page.getByRole("dialog", { name: "Посыльный ворон", exact: true }),
  ).toBeVisible({ timeout: 20000 });
}

async function state(page: Page, fixture: Fixture): Promise<Room> {
  const response = await page.request.get(`/api/rooms/${fixture.id}`);
  expect(response.ok()).toBe(true);
  return response.json();
}
async function confirm(page: Page) {
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Подтвердить решение", exact: true })
    .click();
}

test("private peek requires confirmation, survives reload and RU/EN, and never appears in another browser", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(150000);
  const fixture = fixtures.privacy;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await modal
    .getByRole("button", { name: "Посмотреть карту одичалых", exact: true })
    .click();
  expect((await state(page, fixture)).raven!.stage).toBe("choose");
  expect((await state(page, fixture)).match!.ravenUsed).toBe(false);
  await expect(
    modal.getByRole("heading", {
      name: "Посмотреть карту и потратить Ворона?",
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    modal.getByRole("button", { name: "Подтвердить решение", exact: true }),
  ).toBeFocused();
  await confirm(page);
  await expect(
    modal
      .getByTestId("raven-private-card")
      .getByRole("heading", { name: "Передовой отряд", exact: true }),
  ).toBeVisible({ timeout: 20000 });
  const saved = await state(page, fixture);
  expect(saved.match!.ravenUsed).toBe(true);
  const context = await browser.newContext({ baseURL });
  try {
    const guest = await context.newPage();
    await open(guest, fixture, baseURL!, "greyjoy");
    await expect(
      guest.getByRole("heading", {
        name: "Ожидаем решение дома Старк",
        exact: true,
      }),
    ).toBeVisible();
    await expect(guest.getByTestId("raven-private-card")).toHaveCount(0);
    await expect(guest.locator('img[src*="PreemptiveLarge"]')).toHaveCount(0);
    const foreign = await state(guest, fixture);
    expect(foreign.raven!.card).toBeUndefined();
    expect(foreign.raven!.options).toBeUndefined();
    expect(JSON.stringify(foreign)).not.toContain("preemptive-raid");
    await page.bringToFront();
    await page.reload();
    await expect(modal.getByTestId("raven-private-card")).toBeVisible({
      timeout: 20000,
    });
    expect((await state(page, fixture)).clock.deadline).toBe(
      saved.clock.deadline,
    );
    await modal
      .getByRole("button", { name: "Сменить язык", exact: true })
      .click();
    await expect(
      modal.getByRole("heading", { name: "Messenger Raven", exact: true }),
    ).toBeVisible();
    await expect(
      modal.getByRole("heading", { name: "Preemptive Raid", exact: true }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(modal).toHaveCount(0);
    await page
      .getByRole("button", { name: "Messenger Raven", exact: true })
      .click();
    await expect(modal.getByTestId("raven-private-card")).toBeVisible();
    await page.screenshot({
      path: test.info().outputPath("raven-private-en.png"),
    });
    await modal
      .getByRole("button", { name: "Move to the bottom", exact: true })
      .click();
    await modal
      .getByRole("button", { name: "Confirm decision", exact: true })
      .click();
    await expect
      .poll(async () => (await state(page, fixture)).phase)
      .toBe("marches");
    await expect(modal).toHaveCount(0);
    await guest.bringToFront();
    await expect(guest.getByTestId("raven-private-card")).toHaveCount(0);
    await guest.locator(".decision-next button").click();
    await expect(guest.getByRole("dialog")).toHaveCount(0, { timeout: 20000 });
    expect(JSON.stringify(await state(guest, fixture))).not.toContain(
      "preemptive-raid",
    );
  } finally {
    await context.close();
  }
  expect(errors).toEqual([]);
});

test("tablet order replacement respects server bans and unused tokens, supports map inspection and changes only one order", async ({
  browser,
  baseURL,
}) => {
  test.setTimeout(120000);
  const fixture = fixtures.replacement;
  const context = await browser.newContext({
    baseURL,
    viewport: { width: 820, height: 1180 },
    hasTouch: true,
  });
  try {
    const page = await context.newPage();
    await open(page, fixture, baseURL!);
    const modal = page.getByRole("dialog");
    await modal
      .getByRole("button", { name: "Заменить приказ", exact: true })
      .tap();
    await modal
      .getByLabel("Область с заменяемым приказом", { exact: true })
      .selectOption("winterfell");
    await expect(modal.getByRole("button", { name: /Набег/ })).toHaveCount(0);
    await expect(
      modal.getByRole("button", { name: "Поход −1", exact: true }),
    ).toHaveCount(0);
    await modal
      .getByRole("button", { name: "Показать на карте", exact: true })
      .tap();
    await expect(modal).toHaveCount(0);
    expect((await state(page, fixture)).orders.winterfell).toBe("march");
    await page
      .getByRole("button", { name: "Посыльный ворон", exact: true })
      .tap();
    await modal
      .getByRole("button", { name: "Заменить приказ", exact: true })
      .tap();
    await modal
      .getByLabel("Область с заменяемым приказом", { exact: true })
      .selectOption("winterfell");
    await modal
      .getByRole("button", { name: "Поддержка", exact: true })
      .first()
      .tap();
    expect((await state(page, fixture)).orders.winterfell).toBe("march");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: test.info().outputPath("raven-replacement-tablet.png"),
    });
    await modal
      .getByRole("button", { name: "Подтвердить замену приказа", exact: true })
      .tap();
    await expect
      .poll(async () => (await state(page, fixture)).orders.winterfell)
      .toBe("support-1");
    const final = await state(page, fixture);
    expect(final.orders["white-harbor"]).toBe("march-minus");
    expect(final.match!.ravenUsed).toBe(true);
    expect(final.raven).toBeUndefined();
    await expect(modal).toHaveCount(0);
  } finally {
    await context.close();
  }
});

test("decline is confirmed, cancellable before sending, and does not spend or reveal the Raven", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures.decline;
  const cardRequests: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes("/local-assets/events/"))
      cardRequests.push(request.url());
  });
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await modal
    .getByRole("button", { name: "Отказаться от Ворона", exact: true })
    .click();
  await modal.getByRole("button", { name: "Назад", exact: true }).click();
  expect((await state(page, fixture)).phase).toBe("raven");
  await modal
    .getByRole("button", { name: "Отказаться от Ворона", exact: true })
    .click();
  await confirm(page);
  await expect
    .poll(async () => (await state(page, fixture)).phase)
    .toBe("marches");
  expect((await state(page, fixture)).match!.ravenUsed).toBe(false);
  expect(cardRequests).toEqual([]);
});

test("an empty order area cannot receive a Raven token, but a private peek is still available", async ({
  page,
  baseURL,
}) => {
  const fixture = fixtures["no-orders"];
  await open(page, fixture, baseURL!);
  const modal = page.getByRole("dialog");
  await expect(
    modal.getByRole("button", { name: "Заменить приказ", exact: true }),
  ).toBeDisabled();
  const before = await state(page, fixture);
  const response = await page.request.post(
    `/api/rooms/${fixture.id}/commands`,
    {
      data: {
        type: "raven",
        choice: "replace",
        region: "winterfell",
        order: "support-1",
        revision: before.revision,
      },
    },
  );
  expect(response.status()).toBe(400);
  expect((await state(page, fixture)).revision).toBe(before.revision);
  await modal
    .getByRole("button", { name: "Посмотреть карту одичалых", exact: true })
    .click();
  await confirm(page);
  await expect(modal.getByTestId("raven-private-card")).toBeVisible();
  await modal
    .getByRole("button", { name: "Оставить сверху", exact: true })
    .click();
  await confirm(page);
  await expect
    .poll(async () => (await state(page, fixture)).phase)
    .toBe("marches");
  expect((await state(page, fixture)).orders.winterfell).toBeUndefined();
});
