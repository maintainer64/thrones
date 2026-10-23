import { expect, test } from "@playwright/test";
import { orderModifier, orderTokenArt } from "../src/artwork/order-art";
import type { Catalog, Room } from "../src/types";

for (const width of [1600, 820]) {
  test(`five HD order bases compose all bonuses on the palette and real 3D board at ${width}px`, async ({
    page,
  }) => {
    test.setTimeout(120000);
    await page.setViewportSize({ width, height: 1180 });
    await page.addInitScript(() => {
      localStorage.setItem("westeros-tour-v2", "true");
      localStorage.setItem("westeros-language", "ru");
    });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const brokenOrders: string[] = [];
    page.on("response", (response) => {
      if (
        response.url().includes("/media/orders/") &&
        response.status() >= 400
      ) {
        brokenOrders.push(response.url());
      }
    });
    const created = await page.request.post("/api/rooms", {
      data: { name: "Order art", house: "stark" },
    });
    expect(created.ok()).toBe(true);
    const room: Room = await created.json();
    const path = `/api/rooms/${room.id}`;
    const start = await page.request.post(`${path}/commands`, {
      data: { type: "start", revision: room.revision },
    });
    expect(start.ok(), await start.text()).toBe(true);
    const catalog: Catalog = await page.request
      .get("/api/catalog?lang=ru")
      .then((r) => r.json());
    const hasBoard = (
      await page.request.get("/local-assets/preview3d/game-manifest.json")
    ).ok();
    await page.goto(`/?room=${room.id}`);
    if (hasBoard)
      await expect(page.getByTestId("game3d-board")).toHaveAttribute(
        "data-ready",
        "true",
        { timeout: 60000 },
      );
    const inspector = page.locator(".inspector");
    if (!(await inspector.isVisible())) {
      await page
        .getByRole("button", { name: "Панель выбранной области", exact: true })
        .click();
    }
    const tokens = page.locator(".order-palette .order-token");
    await expect(tokens).toHaveCount(15);
    for (const order of catalog.orders) {
      const token = tokens.locator(
        `xpath=self::*[@data-order-id='${order.id}']`,
      );
      await expect(token).toHaveCount(1);
      await expect(token.locator('[data-symbol="star"]')).toHaveCount(
        order.star ? 1 : 0,
      );
      const modifier = orderModifier(order);
      if (modifier !== undefined)
        await expect(token.locator(".order-token-modifier")).toHaveText(
          modifier,
        );
      else await expect(token.locator(".order-token-modifier")).toHaveCount(0);
      await expect(token.locator("img")).toHaveAttribute(
        "src",
        orderTokenArt(order)!,
      );
      await expect
        .poll(() =>
          token
            .locator("img")
            .evaluate((node) => (node as HTMLImageElement).naturalWidth),
        )
        .toBeGreaterThanOrEqual(1000);
    }
    await inspector.screenshot({
      path: test.info().outputPath("composed-orders.png"),
    });
    const star = catalog.orders.find((order) => order.id === "march-star")!;
    const assign = page
      .locator(".order-palette button")
      .filter({ has: page.locator('[data-order-id="march-star"]') });
    await expect(assign).toHaveAttribute("aria-disabled", "false");
    await assign.click();
    await expect
      .poll(
        async () =>
          (await page.request.get(path).then((r) => r.json())).orders
            .winterfell,
      )
      .toBe("march-star");
    if (hasBoard) {
      const token = page.locator('[data-region3d="winterfell"] .game3d-order');
      await expect(token).toHaveAttribute("data-order-id", "march-star");
      await expect(token.locator('[data-symbol="star"]')).toHaveCount(1);
      await expect(token.locator(".order-token-modifier")).toHaveText("+1");
      await expect(token.locator("img")).toHaveAttribute(
        "src",
        orderTokenArt(star)!,
      );
      await page.screenshot({
        path: test.info().outputPath("starred-march-board.png"),
      });
    }
    // Both the hand and enlarged cards must show genuinely large game pieces,
    // without cutting off symbols or overlapping the ability above them.
    for (const stats of await page
      .locator(".hand-cards .house-card-stats")
      .all()) {
      const box = (await stats.boundingBox())!;
      const parchment = (await stats.locator("..").boundingBox())!;
      const description = (await stats
        .locator("..")
        .locator(".house-card-description")
        .boundingBox())!;
      expect(box.y).toBeGreaterThanOrEqual(
        description.y + description.height - 1,
      );
      expect(box.y + box.height).toBeLessThanOrEqual(
        parchment.y + parchment.height + 1,
      );
      for (const symbol of await stats.locator(".game-symbol").all()) {
        expect((await symbol.boundingBox())!.height).toBeGreaterThanOrEqual(20);
      }
    }
    for (const kind of ["tower", "sword"]) {
      const card = page
        .locator(".hand-cards button")
        .filter({
          has: page.locator(`[data-symbol="${kind}"]`),
        })
        .first();
      await card.click();
      const dialog = page.locator(".card-inspector-modal");
      const symbol = dialog.locator(`[data-symbol="${kind}"]`);
      await expect(symbol).toHaveCount(1);
      expect((await symbol.boundingBox())!.height).toBeGreaterThanOrEqual(35);
      const stats = (await dialog.locator(".house-card-stats").boundingBox())!;
      const description = (await dialog
        .locator(".house-card-description")
        .boundingBox())!;
      const parchment = (await dialog
        .locator(".house-card-parchment")
        .boundingBox())!;
      expect(stats.y).toBeGreaterThanOrEqual(
        description.y + description.height - 1,
      );
      expect(stats.y + stats.height).toBeLessThanOrEqual(
        parchment.y + parchment.height + 1,
      );
      await dialog.screenshot({
        path: test.info().outputPath(`large-${kind}.png`),
      });
      await page.keyboard.press("Escape");
    }
    expect(brokenOrders).toEqual([]);
    expect(errors).toEqual([]);
  });
}
