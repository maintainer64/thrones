import { expect, test, type Page } from "@playwright/test";
import type { Room } from "../src/types";

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_COMBAT_FIXTURES || "{}",
);
test.beforeEach(async ({ page }) => {
  test.skip(
    !process.env.WESTEROS_COMBAT_FIXTURES,
    "Run TestCombatBrowser with real Go HTTP/WS/SQLite",
  );
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
});
async function state(page: Page, fixture: Fixture): Promise<Room> {
  return page.request
    .get(`/api/rooms/${fixture.id}`)
    .then((response) => response.json());
}
async function command(
  page: Page,
  fixture: Fixture,
  house: string,
  body: Record<string, unknown>,
) {
  const cookie = fixture.cookies[house];
  const response = await page.request.post(
    `/api/rooms/${fixture.id}/commands`,
    {
      headers: { Cookie: `${cookie.Name}=${cookie.Value}` },
      data: { ...body, revision: (await state(page, fixture)).revision },
    },
  );
  expect(response.ok(), await response.text()).toBe(true);
}
for (const mode of ["live", "tablet", "english"]) {
  test(`attack context, server strengths, map and end-of-round report: ${mode}`, async ({
    page,
    baseURL,
  }) => {
    test.setTimeout(120000);
    const f = fixtures[mode];
    const ru = mode !== "english";
    if (mode === "tablet")
      await page.setViewportSize({ width: 820, height: 1180 });
    if (mode === "tablet") await page.emulateMedia({ reducedMotion: "reduce" });
    await page.addInitScript(
      (lang) => localStorage.setItem("westeros-language", lang),
      ru ? "ru" : "en",
    );
    const cookie = f.cookies.stark;
    await page
      .context()
      .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL! }]);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/?room=${f.id}`);
    const intro = page.locator(".combat-intro");
    await expect(intro).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole("dialog")).toHaveCount(1);
    await expect(intro.locator(".combat-route")).toContainText(
      ru ? "Винтерфелл" : "Winterfell",
    );
    await expect(intro.locator(".combat-route")).toContainText(
      ru ? "Белая Гавань" : "White Harbor",
    );
    await expect(intro.getByTestId("strength-total")).toHaveText(["9", "7"]);
    await expect(intro.locator('[data-kind="unit"]')).toHaveCount(5);
    await expect(intro.locator('[data-kind="support_unit"]')).toHaveCount(1);
    await expect(intro.locator('[data-kind="garrison"]')).toContainText("2");
    await expect(intro.locator(".combat-remaining")).toContainText("1");
    await intro
      .getByRole("button", {
        name: ru ? "Показать бой на поле" : "Show battle on the board",
        exact: true,
      })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page.locator(".combat-map-banner")).toBeVisible();
    const playback = page.locator(".combat-playback-controls");
    const beforeReplay = (await state(page, f)).revision;
    if (mode === "live") {
      await playback
        .getByRole("button", { name: "Пауза показа", exact: true })
        .click();
      await expect(playback).toHaveAttribute("data-state", "paused");
      await playback
        .getByRole("button", { name: "Продолжить показ", exact: true })
        .click();
    }
    // Art is optional in CI; the route still has a readable static equivalent.
    const board = page.getByTestId("game3d-board");
    if (await board.count()) {
      await expect(board).toHaveAttribute("data-ready", "true", {
        timeout: 60000,
      });
      const skip = playback.getByRole("button", {
        name: ru ? "Пропустить анимацию" : "Skip animation",
        exact: true,
      });
      if (mode === "english" && (await skip.isEnabled())) await skip.click();
      await expect(playback).toHaveAttribute("data-state", "complete", {
        timeout: 30000,
      });
      await expect(page.locator('[data-region3d="winterfell"]')).toHaveClass(
        /battle-from/,
      );
      await expect(page.locator('[data-region3d="white-harbor"]')).toHaveClass(
        /battle-to/,
      );
      await expect(
        page.locator(
          '[data-region3d="white-harbor"] [data-unit-house="stark"]',
        ),
      ).toHaveText("4");
      await expect(
        page.locator(
          '[data-region3d="white-harbor"] [data-unit-house="lannister"]',
        ),
      ).toHaveText("2");
      await expect(
        page.locator('[data-region3d="winterfell"] [data-unit-house="stark"]'),
      ).toHaveText("1");
    }
    expect((await state(page, f)).revision).toBe(beforeReplay);
    await page.screenshot({ path: test.info().outputPath("attack-map.png") });
    await page
      .getByRole("button", {
        name: ru ? "Вернуться к бою" : "Return to battle",
        exact: true,
      })
      .click();
    await intro
      .getByRole("button", {
        name: ru ? "Продолжить" : "Continue",
        exact: true,
      })
      .click();
    const choice = page.locator(".combat-card-choice");
    await expect(choice).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(1);
    await choice
      .locator(".battle-hand button")
      .filter({ hasText: ru ? "Эддард Старк" : "Eddard Stark" })
      .click();
    await command(page, f, "lannister", {
      type: "card",
      card: "ser-kevan-lannister",
    });
    await expect.poll(async () => (await state(page, f)).round).toBe(2);
    const report = page.locator(".combat-report");
    await expect(report).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(1);
    await expect(report.locator(".combat-card-back")).toHaveCount(2);
    await expect(report.locator(".combat-victory-banner")).toHaveCount(0);
    await report
      .getByRole("button", {
        name: ru ? "Раскрыть карты" : "Reveal cards",
        exact: true,
      })
      .click();
    await expect(report.locator(".combat-card-back")).toHaveCount(0);
    await report
      .getByRole("button", {
        name: ru ? "К расчёту силы" : "Calculate strength",
        exact: true,
      })
      .click();
    const fullCalculation = report.getByRole("button", {
      name: ru ? "Показать весь расчёт" : "Show full calculation",
      exact: true,
    });
    if (await fullCalculation.isEnabled()) await fullCalculation.click();
    await expect(report.getByTestId("strength-total")).toHaveText(["13", "8"]);
    expect(
      (await state(page, f)).match!.combats![0].result?.strength?.attacker
        .total,
    ).toBe(13);
    const revision = (await state(page, f)).revision;
    await report
      .getByRole("button", {
        name: ru ? "Показать исход на поле" : "Watch the outcome on the board",
        exact: true,
      })
      .click();
    await expect(playback).toBeVisible();
    const skipLosses = playback.getByRole("button", {
      name: ru ? "Пропустить анимацию" : "Skip animation",
      exact: true,
    });
    if (mode !== "tablet" && (await skipLosses.isEnabled()))
      await skipLosses.click();
    if (await board.count()) {
      await expect(board).toHaveAttribute("data-playback", "losses");
      await expect(
        page.locator(
          '[data-region3d="white-harbor"] [data-unit-house="lannister"]',
        ),
      ).toHaveCount(0);
      await expect(
        page.locator(
          '[data-region3d="white-harbor"] [data-unit-house="stark"]',
        ),
      ).toHaveText("4");
    }
    expect((await state(page, f)).revision).toBe(revision);
    await playback
      .getByRole("button", {
        name: ru ? "Вернуться к бою" : "Return to battle",
        exact: true,
      })
      .click();
    await expect(report.locator(".combat-victory-banner")).toHaveCount(1);
    await page.screenshot({
      path: test.info().outputPath("combat-result.png"),
    });
    await report
      .getByRole("button", {
        name: ru ? "Продолжить" : "Continue",
        exact: true,
      })
      .click();
    await expect(page.locator(".events-modal")).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(1);
    await page.reload();
    await expect(page.locator(".combat-intro")).toHaveCount(0);
    await expect(page.locator(".combat-report")).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}
test("two fast completed battles survive a skipped snapshot and reload, then yield to Westeros", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(120000);
  const f = fixtures.queue;
  const cookie = f.cookies.tyrell; // Public observer of both battles.
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL! }]);
  await page.goto(`/?room=${f.id}`);
  for (const [id, totals] of [
    [1, ["13", "8"]],
    [2, ["4", "3"]],
  ] as const) {
    const intro = page.locator(".combat-intro");
    await expect(intro).toBeVisible({ timeout: 20000 });
    await expect(intro.locator(".eyebrow")).toContainText(`№${id}`);
    if (id === 2) {
      await page.reload();
      await expect(intro.locator(".eyebrow")).toContainText("№2");
    }
    await intro
      .getByRole("button", { name: "Пропустить движение", exact: true })
      .click();
    const report = page.locator(".combat-report");
    await expect(report).toBeVisible();
    await report
      .getByRole("button", { name: "К результату", exact: true })
      .click();
    await expect(report.getByTestId("strength-total")).toHaveText([...totals]);
    await expect(page.getByRole("dialog")).toHaveCount(1);
    await report
      .getByRole("button", { name: "Продолжить", exact: true })
      .click();
  }
  await expect(page.locator(".events-modal")).toBeVisible();
});

test("new battles and Westeros never replace an attack already being read", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(90000);
  const f = fixtures.pinned;
  const cookie = f.cookies.tyrell;
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL! }]);
  await page.goto(`/?room=${f.id}`);
  const intro = page.locator(".combat-intro");
  await expect(intro).toBeVisible({ timeout: 20000 });
  await expect(intro.locator(".eyebrow")).toContainText("№1");
  await intro
    .getByRole("button", { name: "Показать бой на поле", exact: true })
    .click();
  const playback = page.locator(".combat-playback-controls");
  await playback
    .getByRole("button", { name: "Пауза показа", exact: true })
    .click();
  await command(page, f, "greyjoy", { type: "card", card: "dagmar-cleftjaw" });
  await command(page, f, "baratheon", {
    type: "card",
    card: "ser-davos-seaworth",
  });
  await expect.poll(async () => (await state(page, f)).round).toBe(2);
  const board = page.getByTestId("game3d-board");
  if (await board.count()) {
    await expect(board).toHaveAttribute("data-ready", "true", {
      timeout: 60000,
    });
    await expect(playback).toHaveAttribute("data-state", "paused");
    await playback
      .getByRole("button", { name: "Пропустить анимацию", exact: true })
      .click();
    await expect(board).toHaveAttribute("data-playback-pieces", "8");
    // The live first battle already killed these units; replay must retain its initial defenders.
    await expect(
      page.locator(
        '[data-region3d="white-harbor"] [data-unit-house="lannister"]',
      ),
    ).toHaveText("2");
  }
  await playback
    .getByRole("button", { name: "Вернуться к бою", exact: true })
    .click();
  await expect(intro.locator(".eyebrow")).toContainText("№1");
  await expect(page.getByRole("dialog")).toHaveCount(1);
  await intro.getByRole("button", { name: "Продолжить", exact: true }).click();
  await page
    .locator(".combat-report")
    .getByRole("button", { name: "К результату", exact: true })
    .click();
  await expect(
    page.locator(".combat-report").getByTestId("strength-total"),
  ).toHaveText(["13", "8"]);
});

test("one's required card decision is reachable without reading older battles", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(90000);
  const f = fixtures.priority;
  const cookie = f.cookies.greyjoy;
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL! }]);
  await page.goto(`/?room=${f.id}`);
  const intro = page.locator(".combat-intro");
  await expect(intro).toBeVisible({ timeout: 20000 });
  await expect(intro.locator(".eyebrow")).toContainText("№2");
  await intro
    .getByRole("button", { name: "Перейти к вашему решению", exact: true })
    .click();
  const choice = page.locator(".combat-card-choice");
  await expect(choice).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(1);
  await choice
    .locator(".battle-hand button")
    .filter({ hasText: "Дагмер Щербатый" })
    .click();
  await expect
    .poll(async () => (await state(page, f)).battle?.chosen.greyjoy)
    .toBe(true);
});

test("saved casualties and map retreat replay without moving the live army", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(120000);
  const f = fixtures.retreat;
  const cookie = f.cookies.tyrell;
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL! }]);
  await page.goto(`/?room=${f.id}`);
  await page
    .locator(".combat-intro")
    .getByRole("button", { name: "Пропустить движение", exact: true })
    .click({ timeout: 20000 });
  const report = page.locator(".combat-report");
  // Reload does not consume an unread result or leak its cards into the first step.
  await expect(report.locator(".combat-card-back")).toHaveCount(2);
  await page.reload();
  await expect(report.locator(".combat-card-back")).toHaveCount(2, {
    timeout: 20000,
  });
  await report
    .getByRole("button", { name: "К результату", exact: true })
    .click();
  await expect(report.locator(".combat-casualties")).toContainText(
    "Винтерфелл",
  );
  const before = await state(page, f);
  const record = before.match!.combats![0];
  expect(record.result!.retreatTo).toBe("winterfell");
  const lost = Object.values(record.result!.losses!)
    .flat()
    .reduce((sum, unit) => sum + unit.quantity, 0);
  await report
    .getByRole("button", { name: "Показать исход на поле", exact: true })
    .click();
  const board = page.getByTestId("game3d-board");
  const playback = page.locator(".combat-playback-controls");
  if (await board.count()) {
    await expect(board).toHaveAttribute("data-ready", "true", {
      timeout: 60000,
    });
    await expect(playback).toHaveAttribute("data-state", "complete", {
      timeout: 30000,
    });
    await expect(board).toHaveAttribute(
      "data-playback-pieces",
      String(8 - lost),
    );
    await expect(
      page.locator('[data-region3d="winterfell"] [data-unit-house="stark"]'),
    ).toHaveText("4");
    await expect(
      page.locator('[data-region3d="white-harbor"] [data-unit-house="stark"]'),
    ).toHaveCount(0);
    await expect(
      page.locator(
        '[data-region3d="white-harbor"] [data-unit-house="lannister"]',
      ),
    ).toHaveText("2");
    await page.screenshot({
      path: test.info().outputPath("saved-retreat.png"),
    });
  }
  const after = await state(page, f);
  // The response clock advances even when no game command has been sent.
  expect({ ...after, serverTime: before.serverTime }).toEqual(before);
  await playback
    .getByRole("button", { name: "Вернуться к бою", exact: true })
    .click();
  await expect(report.locator(".combat-casualties")).toContainText(
    "Винтерфелл",
  );
});
