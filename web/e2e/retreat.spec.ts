import { test, expect, type APIRequestContext } from "@playwright/test";

for (const touch of [false, true]) {
  test.describe(touch ? "tablet retreat" : "desktop retreat", () => {
    test.use({
      viewport: touch
        ? { width: 1024, height: 768 }
        : { width: 1600, height: 1000 },
      hasTouch: touch,
    });
    test("losing player chooses and confirms a legal retreat on the map", async ({
      page,
      browser,
      baseURL,
    }) => {
      test.setTimeout(120000);
      await page.addInitScript(() =>
        localStorage.setItem("westeros-tour-v2", "true"),
      );
      const response = await page.request.post("/api/rooms", {
        data: { name: "Retreat player" },
      });
      expect(response.ok()).toBe(true);
      const created = await response.json();
      const path = `/api/rooms/${created.id}`;
      const snapshot = () => page.request.get(path).then((r) => r.json());
      const command = async (
        client: APIRequestContext,
        data: Record<string, unknown>,
      ) => {
        const current = await client.get(path).then((r) => r.json());
        const result = await client.post(`${path}/commands`, {
          data: { ...data, revision: current.revision },
        });
        expect(result.ok(), await result.text()).toBe(true);
        return result.json();
      };
      // Six real human seats keep setup deterministic; no mocked room/battle state.
      const guests = [];
      try {
        for (const seat of created.seats.filter(
          (s: { house: string }) => s.house !== created.yourHouse,
        )) {
          await command(page.request, {
            type: "configure",
            house: seat.house,
            bot: false,
            difficulty: "normal",
          });
          const context = await browser.newContext({ baseURL });
          guests.push({ context, house: seat.house });
          const joined = await context.request.post(`${path}/join`, {
            data: { name: seat.house, house: seat.house },
          });
          expect(joined.ok(), await joined.text()).toBe(true);
        }
        await page.goto(`/?room=${created.id}`);
        await expect(page.getByTestId("game3d-board")).toHaveAttribute(
          "data-ready",
          "true",
          { timeout: 60000 },
        );
        await command(page.request, { type: "start" });
        const greyjoy = guests.find((g) => g.house === "greyjoy")!.context
          .request;
        await command(page.request, {
          type: "order",
          region: "winterfell",
          order: "march",
        });
        await command(greyjoy, {
          type: "order",
          region: "greywater-watch",
          order: "march",
        });
        for (const guest of guests)
          await command(guest.context.request, { type: "ready" });
        await command(page.request, { type: "ready" });
        await command(page.request, { type: "continue" });
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
        await command(page.request, { type: "card", card: "catelyn-stark" });
        await command(greyjoy, { type: "card", card: "victarion-greyjoy" });
        const before = await snapshot();
        expect(before.phase).toBe("retreat");
        expect(before.battle.loser).toBe("stark");
        expect(before.battle.retreats).toContain("winterfell");
        const panel = page.getByTestId("map-retreat-panel");
        await expect(panel).toBeVisible();
        await expect(page.getByRole("dialog")).toHaveCount(0);
        const confirm = panel.getByRole("button", {
          name: "Подтвердить отступление",
          exact: true,
        });
        await expect(confirm).toBeDisabled();
        await page.reload();
        await expect(page.getByTestId("game3d-board")).toHaveAttribute(
          "data-ready",
          "true",
          { timeout: 60000 },
        );
        await expect(panel).toBeVisible();
        await expect(page.getByRole("dialog")).toHaveCount(0);
        await expect(confirm).toBeDisabled();
        const target = page.locator('[data-region3d="winterfell"]');
        await expect(target).toHaveAttribute("data-eligible", "true");
        // A non-target may be inspected but must never become a retreat destination.
        await page.locator('[data-region3d="moat-cailin"]').click();
        await expect(confirm).toBeDisabled();
        await panel
          .getByRole("button", { name: "Показать цели", exact: true })
          .click();
        if (touch) await target.tap({ timeout: 15000 });
        else await target.click({ timeout: 15000 });
        await expect(target).toHaveClass(/target/);
        await expect(panel).toContainText("Винтерфелл");
        await expect(confirm).toBeEnabled();
        await expect(confirm).toBeInViewport();
        expect((await snapshot()).revision).toBe(before.revision);
        await page.screenshot({
          path: "test-results/map-retreat-selected.png",
        });
        await confirm.click();
        await expect
          .poll(async () => (await snapshot()).phase)
          .not.toBe("retreat");
        const after = await snapshot();
        expect(after.match.armies.winterfell).toEqual(
          expect.arrayContaining([
            { unitType: "footman", house: "stark", quantity: 1 },
          ]),
        );
        expect(after.events).toEqual(
          expect.arrayContaining([
            expect.objectContaining({
              type: "retreat",
              house: "stark",
              from: "moat-cailin",
              to: "winterfell",
            }),
          ]),
        );
        await expect(panel).toHaveCount(0);
      } finally {
        await Promise.all(guests.map((guest) => guest.context.close()));
      }
    });
  });
}
