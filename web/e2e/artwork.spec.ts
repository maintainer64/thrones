import { expect, test } from "@playwright/test";
import { createHash } from "node:crypto";
import sharp from "sharp";
import { artwork, artworkURL, originalPortrait } from "../src/artwork/artwork";
import { textureKey } from "../src/cards/card-art";
import type { Catalog } from "../src/types";

test("all indexed WebP renditions have matching revisions and original dimensions", async ({
  request,
}) => {
  for (const [path, image] of Object.entries(artwork)) {
    const response = await request.get(artworkURL(path));
    test.skip(response.status() === 404, "Local artwork is optional in CI");
    expect(response.ok(), path).toBe(true);
    expect(response.headers()["content-type"]).toContain("image/webp");
    const bytes = await response.body();
    expect(
      createHash("sha256").update(bytes).digest("hex").slice(0, 12),
      path,
    ).toBe(image.revision);
    expect(bytes.length, path).toBe(image.bytes);
    const decoded = await sharp(bytes).metadata();
    expect(decoded.format).toBe("webp");
    expect(decoded.width, path).toBe(image.width);
    expect(decoded.height, path).toBe(image.height);
    await response.dispose();
  }
});

test("all six houses use HD leaders and cards, including square Brienne on a tablet", async ({
  page,
}) => {
  test.setTimeout(90000);
  test.skip(
    (await page.request.get(artworkURL("T_crd_stark_eddard.png"))).status() ===
      404,
    "Local artwork is optional in CI",
  );
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const legacyRequests: string[] = [];
  page.on("request", (request) => {
    if (
      /\/(media\/cards\/|local-assets\/cards\/|api\/local-assets(?:\?|$))/.test(
        request.url(),
      )
    ) {
      legacyRequests.push(request.url());
    }
  });
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  // Keep the real room, catalog and artwork; this suite doesn't exercise WebGL.
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "artwork-only test" }),
  );
  const catalog: Catalog = await page.request
    .get("/api/catalog?lang=ru")
    .then((r) => r.json());
  for (const house of [
    "stark",
    "greyjoy",
    "lannister",
    "tyrell",
    "martell",
    "baratheon",
  ]) {
    const response = await page.request.post("/api/rooms", {
      data: { name: "HD artwork", house },
    });
    expect(response.ok(), await response.text()).toBe(true);
    const room = await response.json();
    await page.goto(`/?room=${room.id}`);
    await expect(page.locator(".hand-cards .house-card-art")).toHaveCount(7);
    for (const card of Object.keys(catalog.houses[house].houseCards)) {
      const src = originalPortrait(textureKey(card, house))!;
      expect(src, `${house}/${card}`).toBeTruthy();
      // Each card must use its new original, not a stale cards/*.webp derivative.
      const original = page.locator(`.hand-cards img[src="${src}"]`);
      await expect(original).toHaveCount(1);
      await expect
        .poll(() =>
          original.evaluate((node) => (node as HTMLImageElement).naturalWidth),
        )
        .toBeGreaterThanOrEqual(1000);
      const stats = original.locator("../..").locator(".house-card-stats");
      if (await stats.count()) {
        const box = (await stats.boundingBox())!;
        const parchment = (await stats.locator("..").boundingBox())!;
        const description = (await stats
          .locator("..")
          .locator(".house-card-description")
          .boundingBox())!;
        expect(
          box.y,
          `${house}/${card}: ability stays above symbols`,
        ).toBeGreaterThanOrEqual(description.y + description.height - 1);
        expect(
          box.y + box.height,
          `${house}/${card}: symbols fit on parchment`,
        ).toBeLessThanOrEqual(parchment.y + parchment.height + 1);
        for (const symbol of await stats.locator(".game-symbol").all()) {
          expect((await symbol.boundingBox())!.height).toBeGreaterThanOrEqual(
            20,
          );
        }
      }
    }
    await page
      .getByRole("button", { name: "Панель великих домов", exact: true })
      .click();
    await expect(page.locator(".houses .seat-portrait")).toHaveCount(6);
    for (const img of await page.locator(".houses .seat-portrait").all()) {
      await expect(img).toHaveAttribute(
        "src",
        /^\/local-assets\/T_crd_.+\.webp\?v=[a-f0-9]{12}$/,
      );
      await expect
        .poll(() =>
          img.evaluate((node) => (node as HTMLImageElement).naturalWidth),
        )
        .toBeGreaterThanOrEqual(1000);
    }
    if (house === "stark") {
      await page.screenshot({
        path: test.info().outputPath("hd-leaders-and-hand.png"),
      });
    }
    await page
      .getByRole("button", { name: "Панель великих домов", exact: true })
      .click();
    if (house === "baratheon") {
      await page.setViewportSize({ width: 820, height: 1180 });
      const brienne = page.locator(".hand-cards button").filter({
        has: page.locator('img[src*="T_crd_baratheon_brienne.webp"]'),
      });
      await brienne.click();
      const dialog = page.locator(".card-inspector-modal");
      await expect(dialog).toBeVisible();
      const portrait = dialog.locator(".house-card-art");
      await expect(portrait).toHaveAttribute(
        "src",
        originalPortrait("T_crd_baratheon_brienne")!,
      );
      await expect
        .poll(() =>
          portrait.evaluate((node) => (node as HTMLImageElement).naturalWidth),
        )
        .toBe(1254);
      const cardBox = (await dialog.locator(".house-card").boundingBox())!;
      expect(cardBox.y).toBeGreaterThanOrEqual(0);
      expect(cardBox.y + cardBox.height).toBeLessThanOrEqual(1180);
      await page.screenshot({
        path: test.info().outputPath("hd-brienne-tablet.png"),
      });
      await page.keyboard.press("Escape");
      await page.setViewportSize({ width: 1600, height: 1000 });
    }
  }
  expect(errors).toEqual([]);
  expect(legacyRequests).toEqual([]);
});
