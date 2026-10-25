import { expect, test, type Locator, type Page } from "@playwright/test";
import type { Room } from "../src/types";

async function noHorizontalOverflow(panel: Locator) {
  await expect(panel).toBeVisible();
  expect(
    await panel.evaluate((node) => node.scrollWidth <= node.clientWidth + 1),
  ).toBe(true);
}
async function snapshot(page: Page): Promise<Room> {
  const id = new URL(page.url()).searchParams.get("room");
  return page.request
    .get(`/api/rooms/${id}`)
    .then((response) => response.json());
}

for (const language of ["ru", "en"] as const) {
  for (const width of [1600, 820, 390]) {
    test(`readable houses and right panels ${language} at ${width}px`, async ({
      page,
    }) => {
      test.setTimeout(120000);
      await page.setViewportSize({
        width,
        height: width === 1600 ? 1000 : 1180,
      });
      await page.addInitScript((lang) => {
        localStorage.setItem("westeros-tour-v2", "true");
        localStorage.setItem("westeros-language", lang);
      }, language);
      const ru = language === "ru";
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      const manifestResponse = page.waitForResponse((response) =>
        response.url().endsWith("/local-assets/preview3d/game-manifest.json"),
      );
      await page.goto("/");
      const hasLocalArt = (await manifestResponse).ok();
      await page
        .getByLabel(ru ? "Имя игрока" : "Player name", { exact: true })
        .fill(
          ru ? "Игрок с очень длинным именем" : "Player with a very long name",
        );
      await page
        .getByRole("button", {
          name: ru ? "Собрать малый совет" : "Gather the Small Council",
          exact: true,
        })
        .click();
      const houses = page.locator(".houses");
      // Proprietary local artwork is not installed in CI. Panel checks still
      // use the real Go game state; wait for the 3D scene only when it exists.
      if (hasLocalArt)
        await expect(page.getByTestId("game3d-board")).toHaveAttribute(
          "data-ready",
          "true",
          { timeout: 60000 },
        );
      await page
        .getByRole("button", {
          name: ru ? "Панель великих домов" : "Great houses panel",
          exact: true,
        })
        .click();
      await noHorizontalOverflow(houses);
      await expect(houses.locator(".seat")).toHaveCount(6);
      for (const label of await houses
        .locator(".house-picker-option > span")
        .all()) {
        expect(
          await label.evaluate((node) => node.scrollWidth <= node.clientWidth),
        ).toBe(true);
        expect((await label.boundingBox())!.height).toBeLessThan(25);
      }
      expect(
        await houses
          .locator(".seat-resources small")
          .first()
          .evaluate((node) => getComputedStyle(node).display),
      ).toBe("inline");
      for (const name of await houses.locator(".seat-title b").all()) {
        expect(
          await name.evaluate((node) => node.getBoundingClientRect().height),
        ).toBeLessThan(25);
      }
      const identity = houses.locator(".seat.you .seat-name > span");
      expect(
        (await identity.getAttribute("title")) ||
          (await identity.locator("..").getAttribute("title")),
      ).toContain(ru ? "длинным" : "very long");
      // Actual server commands, not fixture data: bot controls must stay usable.
      const greyjoy = houses
        .locator(".seat")
        .filter({ has: page.locator('.seat-heraldry[src$="greyjoy.svg"]') });
      await greyjoy
        .getByLabel(ru ? "Сложность бота" : "Bot difficulty", { exact: true })
        .selectOption("hard");
      await expect
        .poll(
          async () =>
            (await snapshot(page)).seats.find(
              (seat) => seat.house === "greyjoy",
            )?.difficulty,
        )
        .toBe("hard");
      const selects = greyjoy.locator("select");
      for (const select of await selects.all()) {
        const box = (await select.boundingBox())!;
        expect(box.width).toBeGreaterThan(100);
        expect(box.x + box.width).toBeLessThanOrEqual(width);
      }
      await page.screenshot({ path: test.info().outputPath("houses.png") });
      await page
        .getByRole("button", {
          name: ru ? "Панель великих домов" : "Great houses panel",
          exact: true,
        })
        .click();
      await page
        .getByRole("button", {
          name: ru ? "Начать партию" : "Start game",
          exact: true,
        })
        .click();
      await expect
        .poll(async () => (await snapshot(page)).phase)
        .toBe("planning");
      if (!(await page.locator(".inspector").isVisible()))
        await page
          .getByRole("button", {
            name: ru ? "Панель выбранной области" : "Selected area panel",
            exact: true,
          })
          .click();
      const inspector = page.locator(".inspector");
      await noHorizontalOverflow(inspector);
      const state = await snapshot(page);
      const budget = inspector.getByRole("region", {
        name: ru ? "Власть и особые приказы" : "Power and special orders",
        exact: true,
      });
      await expect(budget.locator("dd").first()).toHaveText(
        String(state.orderBudget!.power),
      );
      await expect(budget.locator("dd").last()).toHaveText(
        `${state.orderBudget!.starsAllowed} / ${state.orderBudget!.starsAllowed}`,
      );
      const fortification = inspector.locator(".fortification-info details");
      await expect(fortification).not.toHaveAttribute("open");
      expect((await fortification.boundingBox())!.height).toBeLessThan(50);
      await fortification.locator("summary").click();
      await expect(fortification.locator("p")).toBeVisible();
      await fortification.locator("summary").click();
      await expect(inspector.locator(".order-palette button")).toHaveCount(15);
      await page.screenshot({ path: test.info().outputPath("area.png") });
      await page
        .getByRole("button", {
          name: ru ? "Панель дорожек влияния" : "Influence tracks panel",
          exact: true,
        })
        .click();
      const tracks = page.locator(".influence-panel");
      await noHorizontalOverflow(tracks);
      const tops: number[] = [];
      for (const column of await tracks
        .locator(".influence-columns section")
        .all()) {
        const heading = column.locator("h3");
        expect(
          await heading.evaluate(
            (node) => node.scrollWidth <= node.clientWidth,
          ),
        ).toBe(true);
        tops.push((await column.locator("li").first().boundingBox())!.y);
      }
      expect(Math.max(...tops) - Math.min(...tops)).toBeLessThan(1);
      const stars = tracks.locator(".track-stars").first();
      const starBox = (await stars.boundingBox())!;
      const rankBox = (await stars
        .locator("..")
        .locator("small")
        .first()
        .boundingBox())!;
      expect(rankBox.y + rankBox.height).toBeLessThanOrEqual(starBox.y);
      await page.screenshot({ path: test.info().outputPath("influence.png") });
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      expect(errors).toEqual([]);
    });
  }
}
