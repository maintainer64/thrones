import { test, expect } from "@playwright/test";
import { openGameMenu, switchLanguage } from "./helpers/game-menu";

test.beforeEach(async ({ page }) => {
  // Card tests need real room data, not the expensive independent WebGL scene.
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "card-only test" }),
  );
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
});

test("frameless cards, large names, zoom, keyboard navigation and focus restoration", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("Card viewer");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await expect(page.locator(".hand-cards .house-card")).toHaveCount(7);
  await page.locator(".hand-cards button").first().click();
  const dialog = page.locator(".card-inspector-modal");
  const card = dialog.locator(".house-card");
  await expect(dialog).toBeVisible();
  await expect(dialog.locator(".house-card-frame")).toHaveCount(0);
  await expect(dialog.locator(".house-card-description")).not.toBeEmpty();
  await expect(dialog.locator(".house-card-art")).toHaveJSProperty(
    "naturalWidth",
    344,
  );
  const initial = (await card.boundingBox())!;
  expect(initial.width).toBeGreaterThanOrEqual(300);
  await card.hover({ position: { x: 45, y: 100 } });
  await expect
    .poll(() =>
      card.evaluate((element) => element.style.getPropertyValue("--tilt-y")),
    )
    .not.toBe("0deg");
  await page
    .getByRole("button", { name: "Увеличить карту", exact: true })
    .click();
  await expect(page.getByLabel("Текущий масштаб")).toHaveText("120%");
  expect((await card.boundingBox())!.width).toBeGreaterThan(initial.width);
  await page.getByRole("button", { name: "Сбросить масштаб карты" }).click();
  const originalName = await dialog.locator("h2").innerText();
  await page.keyboard.press("ArrowRight");
  await expect(dialog.locator("h2")).not.toHaveText(originalName);
  await page.keyboard.press("ArrowLeft");
  await expect(dialog.locator("h2")).toHaveText(originalName);
  await page.screenshot({
    path: "test-results/card-inspector.png",
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(page.locator(".hand-cards button").first()).toBeFocused();
  await openGameMenu(page);
  await page.getByRole("button", { name: "Анимации", exact: true }).click();
  await page.locator(".hand-cards button").first().click();
  await expect(card).not.toHaveClass(/animated/);
  await card.hover({ position: { x: 45, y: 100 } });
  expect(
    await card.evaluate((element) =>
      element.style.getPropertyValue("--tilt-y"),
    ),
  ).toBe("0deg");
  expect(errors).toEqual([]);
});

test("cards remain usable without optional PSD assets and respect reduced motion", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.route("**/local-assets/ui/strength-shield.png", (route) =>
    route.fulfill({ status: 404, body: "optional shield unavailable" }),
  );
  await page.route("**/local-assets/cards/manifest.json", (route) =>
    route.fulfill({ status: 404, body: "not imported" }),
  );
  await page.goto("/");
  await page
    .getByLabel("Имя игрока", { exact: true })
    .fill("Fallback card viewer");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await page.locator(".hand-cards button").first().click();
  const card = page.locator(".card-inspector-stage .house-card");
  await expect(card).not.toHaveClass(/has-frame|animated/);
  await expect(card.locator(".strength-shield")).toHaveClass(/shield-fallback/);
  await expect(card.locator(".house-card-art")).toHaveJSProperty(
    "naturalWidth",
    512,
  );
  await page
    .getByRole("button", { name: "Следующая карта", exact: true })
    .click();
  await expect(page.locator(".card-inspector-navigation")).toContainText(
    "2 / 7",
  );
});

test("mobile zoom scrolls inside the viewer without overflowing the page", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("Mobile cards");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  await page.locator(".hand-cards button").first().click();
  for (let index = 0; index < 5; index++)
    await page
      .getByRole("button", { name: "Увеличить карту", exact: true })
      .click();
  await expect(page.getByLabel("Текущий масштаб")).toHaveText("200%");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(
    await page
      .locator(".card-inspector-stage")
      .evaluate((element) => element.scrollWidth > element.clientWidth),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/card-mobile.png",
    fullPage: false,
  });
  const box = (await page.locator(".card-inspector-modal").boundingBox())!;
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.y + box.height).toBeLessThanOrEqual(844);
  await expect(
    page.getByRole("button", { name: "Закрыть карту", exact: true }),
  ).toBeInViewport();
});

test("all 42 cards use full-width portraits and readable parchment abilities in RU and EN", async ({
  page,
  browser,
  baseURL,
}, testInfo) => {
  test.setTimeout(120000);
  const created = await page.request
    .post("/api/rooms", { data: { name: "Portrait framing" } })
    .then((response) => response.json());
  const roomPath = `/api/rooms/${created.id}`;
  for (const seat of created.seats as { house: string }[]) {
    const context = await browser.newContext({
      baseURL,
      reducedMotion: "reduce",
    });
    try {
      const viewer =
        seat.house === created.yourHouse ? page : await context.newPage();
      if (viewer !== page) {
        const state = await page.request.get(roomPath).then((r) => r.json());
        const configured = await page.request.post(`${roomPath}/commands`, {
          data: {
            type: "configure",
            revision: state.revision,
            house: seat.house,
            bot: false,
            difficulty: "normal",
          },
        });
        expect(configured.ok(), await configured.text()).toBe(true);
        const joined = await context.request.post(`${roomPath}/join`, {
          data: { name: seat.house, house: seat.house },
        });
        expect(joined.ok()).toBe(true);
        await viewer.addInitScript(() =>
          localStorage.setItem("westeros-tour-v2", "true"),
        );
        await viewer.route(
          "**/local-assets/preview3d/game-manifest.json",
          (route) => route.fulfill({ status: 404, body: "card-only test" }),
        );
      }
      await viewer.goto(`/?room=${created.id}`);
      const hand = viewer.locator(".hand-cards .house-card");
      await expect(hand).toHaveCount(7);
      const checkFit = async (selector: string) => {
        const cards = viewer.locator(selector);
        for (const card of await cards.all()) {
          await expect(card.locator(".house-card-art")).toHaveJSProperty(
            "naturalWidth",
            344,
          );
          const fit = await card.evaluate((element) => {
            const portrait = element.querySelector(".house-card-portrait")!;
            const art = element.querySelector(".house-card-art")!;
            const badge = element.querySelector(".house-card-strength")!;
            const parchment = element.querySelector(".house-card-parchment")!;
            const description = element.querySelector(
              ".house-card-description",
            )!;
            const title = element.querySelector(".house-card-name")!;
            const box = element.getBoundingClientRect();
            const window = portrait.getBoundingClientRect();
            const strength = badge.getBoundingClientRect();
            return {
              top: (window.top - box.top) / box.height,
              height: window.height / box.height,
              strengthX: (strength.left - box.left) / box.width,
              strengthY: (strength.top - box.top) / box.height,
              fit: getComputedStyle(art).objectFit,
              animation: getComputedStyle(art).animationName,
              overflow: getComputedStyle(portrait).overflow,
              width: window.width / box.width,
              textFits: parchment.scrollHeight <= parchment.clientHeight + 1,
              nameFits: title.scrollHeight <= title.clientHeight + 1,
              text: description.textContent,
              font:
                parseFloat(getComputedStyle(title).fontSize) /
                element.clientWidth,
              rulesTop:
                parseFloat(
                  (element as HTMLElement).style.getPropertyValue(
                    "--card-rules-top",
                  ),
                ) / 100,
              descriptionFont:
                parseFloat(getComputedStyle(description).fontSize) /
                element.clientWidth,
            };
          });
          expect(fit.top).toBeCloseTo(0.11, 2);
          expect(fit.height).toBeCloseTo(fit.rulesTop - 0.09, 2);
          expect(fit.width).toBeCloseTo(1, 2);
          expect(fit.strengthX).toBeCloseTo(0.01, 2);
          expect(fit.strengthY).toBeCloseTo(-0.005, 2);
          expect(fit.fit).toBe("cover");
          expect(fit.font).toBeCloseTo(0.075, 2);
          expect(fit.descriptionFont).toBeCloseTo(0.052, 2);
          expect(fit.textFits, fit.text || "ability").toBe(true);
          expect(fit.nameFits).toBe(true);
          expect(fit.overflow).toBe("hidden");
        }
      };
      await checkFit(".hand-cards .house-card");
      await viewer.locator(".hand-cards button").first().click();
      for (let index = 0; index < 7; index++) {
        await checkFit(".card-inspector-modal .house-card");
        await viewer
          .getByRole("button", { name: "Следующая карта", exact: true })
          .click();
      }
      await viewer
        .getByRole("button", { name: "Закрыть карту", exact: true })
        .click();
      await switchLanguage(viewer);
      await expect(viewer.locator("html")).toHaveAttribute("lang", "en");
      await checkFit(".hand-cards .house-card");
      await viewer.locator(".hand-cards button").first().click();
      for (let index = 0; index < 7; index++) {
        await checkFit(".card-inspector-modal .house-card");
        await viewer
          .getByRole("button", { name: "Next card", exact: true })
          .click();
      }
      await viewer.screenshot({
        path: testInfo.outputPath(`${seat.house}-frame.png`),
      });
      await viewer.keyboard.press("Escape");
      await viewer
        .locator(".hand-cards")
        .screenshot({ path: testInfo.outputPath(`${seat.house}-hand.png`) });
    } finally {
      await context.close();
    }
  }
});

for (const viewport of [
  { width: 1600, height: 900 },
  { width: 1366, height: 650 },
  { width: 390, height: 844 },
  { width: 844, height: 390 },
]) {
  test(`100% shows the complete card at ${viewport.width}×${viewport.height}`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/");
    await page
      .getByLabel("Имя игрока", { exact: true })
      .fill("Fit card viewer");
    await page.getByRole("button", { name: "Собрать малый совет" }).click();
    await page.locator(".hand-cards button").first().click();
    await page.evaluate(() => document.fonts.ready);
    const stage = page.locator(".card-inspector-stage");
    const card = stage.locator(".house-card");
    const verify = async () => {
      const box = (await card.boundingBox())!;
      const viewport = (await stage.boundingBox())!;
      const copy = (await page.locator(".card-inspector-copy").boundingBox())!;
      // The two regions must not overlap, regardless of the grid orientation.
      expect(
        viewport.x + viewport.width <= copy.x + 1 ||
          viewport.y + viewport.height <= copy.y + 1,
      ).toBe(true);
      expect(box.x).toBeGreaterThanOrEqual(viewport.x);
      expect(box.y).toBeGreaterThanOrEqual(viewport.y);
      expect(box.x + box.width).toBeLessThanOrEqual(
        viewport.x + viewport.width + 1,
      );
      expect(box.y + box.height).toBeLessThanOrEqual(
        viewport.y + viewport.height + 1,
      );
      expect(Math.abs(box.height / box.width - 1.7)).toBeLessThan(0.01);
    };
    await verify();
    for (let index = 0; index < 6; index++) {
      await page.keyboard.press("ArrowRight");
      await verify();
    }
    await page.screenshot({ path: testInfo.outputPath("whole-card.png") });
    await page
      .getByRole("button", { name: "Увеличить карту", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Сбросить масштаб карты", exact: true })
      .click();
    await verify();
  });
}
