import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import type { Room } from "../src/types";

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_PROVISION_FIXTURES || "{}",
);
test.skip(
  !process.env.WESTEROS_PROVISION_FIXTURES,
  "Run TestProvisionBrowser against real Go HTTP/WS and temporary SQLite",
);
async function open(page: Page, key: string, baseURL: string) {
  const fixture = fixtures[key],
    cookie = fixture.cookies.stark;
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL }]);
  await page.addInitScript(() => {
    localStorage.setItem("westeros-tour-v2", "true");
    localStorage.setItem("westeros-language", "ru");
  });
  await page.goto(`${baseURL}/?room=${fixture.id}`);
  await expect(page.getByRole("dialog")).toBeVisible({ timeout: 20000 });
  return async (): Promise<Room> => {
    const response = await page.request.get(`/api/rooms/${fixture.id}`);
    expect(response.ok()).toBe(true);
    return response.json();
  };
}

test("supply disband confirmation, real army update and selection on the board", async ({
  page,
  baseURL,
}) => {
  const state = await open(page, "supply", baseURL!);
  const modal = page.getByRole("dialog");
  await expect(
    modal.getByRole("heading", { name: "Недостаточно снабжения", exact: true }),
  ).toBeVisible();
  await modal
    .getByRole("navigation", { name: "Области решения" })
    .getByRole("button", { name: /Винтерфелл/ })
    .click();
  await modal
    .getByRole("button", { name: "Выбрать область на поле", exact: true })
    .click();
  await expect(modal).toHaveCount(0);
  await expect(page.locator(".inspector h2")).toHaveText("Винтерфелл");
  await page
    .getByRole("button", { name: "Снабжение и войска", exact: true })
    .click();
  await modal
    .getByRole("navigation", { name: "Области решения" })
    .getByRole("button", { name: /Винтерфелл/ })
    .click();
  await modal.getByLabel("Расформировать: Пехота", { exact: true }).fill("1");
  await modal
    .getByRole("button", { name: "Расформировать выбранных", exact: true })
    .click();
  expect((await state()).match!.armies.winterfell[0].quantity).toBe(3);
  await modal
    .getByRole("button", { name: "Продолжить выбор", exact: true })
    .click();
  await page.reload();
  await expect(
    modal.getByRole("heading", { name: "Недостаточно снабжения", exact: true }),
  ).toBeVisible();
  await modal
    .getByRole("navigation", { name: "Области решения" })
    .getByRole("button", { name: /Винтерфелл/ })
    .click();
  await modal.getByLabel("Расформировать: Пехота", { exact: true }).fill("1");
  await modal
    .getByRole("button", { name: "Расформировать выбранных", exact: true })
    .click();
  await page.screenshot({ path: test.info().outputPath("supply-confirm.png") });
  await modal
    .getByRole("button", { name: "Подтвердить расформирование", exact: true })
    .click();
  await expect(modal).toHaveCount(0);
  const after = await state();
  expect(after.phase).toBe("westeros");
  expect(after.supply).toBeUndefined();
  expect(after.match!.supply.stark).toBe(1);
  expect(
    after.match!.armies.winterfell.reduce((n, u) => n + u.quantity, 0),
  ).toBe(3);
});

test("mustering spends per-area points, upgrades figures and recruits into a port beside an enemy sea", async ({
  page,
  baseURL,
}) => {
  const state = await open(page, "muster", baseURL!);
  const modal = page.getByRole("dialog");
  await modal
    .getByRole("navigation", { name: "Области решения" })
    .getByRole("button", { name: /Винтерфелл/ })
    .click();
  await modal.getByRole("button", { name: /Улучшить пехоту → Рыцари/ }).click();
  await expect
    .poll(async () => (await state()).mustering!.points.winterfell)
    .toBe(1);
  await page.reload();
  await expect(
    modal.getByRole("heading", { name: "Сбор войск", exact: true }),
  ).toBeVisible();
  await modal
    .getByRole("navigation", { name: "Области решения" })
    .getByRole("button", { name: /Винтерфелл/ })
    .click();
  await modal
    .locator(".provision-options button")
    .filter({ hasText: /^Пехота/ })
    .click();
  await expect
    .poll(async () => (await state()).mustering!.points.winterfell)
    .toBe(0);
  await modal
    .getByRole("navigation", { name: "Области решения" })
    .getByRole("button", { name: /Белая Гавань/ })
    .click();
  await expect(
    modal
      .locator(".provision-options button")
      .filter({ hasText: "Узкое море" }),
  ).toHaveCount(0);
  const ship = modal
    .locator(".provision-options button")
    .filter({ hasText: "Корабли" })
    .filter({ hasText: "Порт Белой Гавани" });
  await expect(ship).toBeEnabled();
  await page.screenshot({ path: test.info().outputPath("muster-options.png") });
  await ship.click();
  await expect(modal).toHaveCount(0);
  const after = await state();
  expect(after.phase).toBe("westeros");
  expect(after.match!.armies["port-of-white-harbor"][0].quantity).toBe(1);
  expect(
    after.match!.armies.winterfell.find((u) => u.unitType === "knight")!
      .quantity,
  ).toBe(1);
  expect(
    after.match!.armies.winterfell.find((u) => u.unitType === "footman")!
      .quantity,
  ).toBe(2);
});

test("tablet starred order chooses local mustering and confirms loss of unused points before next round", async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({
    viewport: { width: 768, height: 1024 },
    hasTouch: true,
  });
  try {
    const page = await context.newPage();
    const state = await open(page, "power", baseURL!);
    const modal = page.getByRole("dialog");
    await expect(
      modal.getByRole("heading", { name: "Особый сбор власти", exact: true }),
    ).toBeVisible();
    await modal
      .getByRole("button", {
        name: "Собрать войска вместо власти",
        exact: true,
      })
      .tap();
    await expect(
      modal.getByRole("heading", { name: "Сбор войск", exact: true }),
    ).toBeVisible();
    await expect(
      modal.getByRole("navigation", { name: "Области решения" }),
    ).toBeVisible();
    expect(Object.keys((await state()).mustering!.points)).toEqual([
      "winterfell",
    ]);
    await modal
      .getByRole("button", { name: "Завершить сбор войск", exact: true })
      .tap();
    expect((await state()).phase).toBe("mustering");
    await modal
      .getByRole("button", { name: "Продолжить выбор", exact: true })
      .tap();
    await page.reload();
    await expect(
      modal.getByRole("heading", { name: "Сбор войск", exact: true }),
    ).toBeVisible();
    await modal
      .getByRole("button", { name: "Завершить сбор войск", exact: true })
      .tap();
    await page.screenshot({
      path: test.info().outputPath("muster-tablet-finish.png"),
    });
    await modal
      .getByRole("button", {
        name: "Завершить и потерять остаток очков",
        exact: true,
      })
      .tap();
    await expect(
      modal.getByRole("heading", { name: "События Вестероса", exact: true }),
    ).toBeVisible();
    const after = await state();
    expect(after.phase).toBe("westeros");
    expect(after.round).toBe(2);
    expect(after.match!.power.stark).toBe(5);
  } finally {
    await context.close();
  }
});

test("starred power can collect instead of mustering and RU/EN preserves the pending decision", async ({
  page,
  baseURL,
}) => {
  const state = await open(page, "power-collect", baseURL!);
  const modal = page.getByRole("dialog");
  await modal
    .getByRole("button", { name: "Сменить язык", exact: true })
    .click();
  await expect(
    modal.getByRole("heading", {
      name: "Special Consolidate Power",
      exact: true,
    }),
  ).toBeVisible();
  await modal
    .getByRole("button", { name: "Collect power: +2", exact: true })
    .click();
  await expect(
    modal.getByRole("heading", { name: "Westeros events", exact: true }),
  ).toBeVisible();
  const after = await state();
  expect(after.phase).toBe("westeros");
  expect(after.round).toBe(2);
  expect(after.match!.power.stark).toBe(7);
  expect(after.mustering).toBeUndefined();
});
