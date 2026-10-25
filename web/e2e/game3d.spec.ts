import { expect, test } from "@playwright/test";

test("relief table pans at a fixed angle and validates a map-first partial march", async ({
  page,
}) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() => {
    localStorage.setItem("westeros-tour-v2", "true");
    localStorage.setItem("westeros-board-mode", "2d"); // Obsolete preferences must not bring 2D back.
  });
  await page.goto("/");
  await page.getByLabel("Имя игрока", { exact: true }).fill("3D player");
  await page.getByRole("button", { name: "Собрать малый совет" }).click();
  const board = page.getByTestId("game3d-board");
  await expect(board).toHaveAttribute("data-ready", "true", { timeout: 60000 });
  await expect(board).toHaveAttribute("data-piece-count", "27");
  await expect(page.locator("[data-region3d]")).toHaveCount(58);
  await expect(board).toHaveAttribute("data-border-count", "58");
  expect(Number(await board.getAttribute("data-relief-max"))).toBeGreaterThan(
    0.4,
  );
  await expect(board).toHaveAttribute("data-camera-polar", "0.785");
  await page.waitForTimeout(1800);
  await page.screenshot({ path: "test-results/game3d-table.png" });
  const canvas = page.locator("canvas"),
    canvasBox = await canvas.boundingBox();
  const oldTarget = await board.getAttribute("data-camera-target");
  await page.mouse.move(
    canvasBox!.x + 75,
    canvasBox!.y + canvasBox!.height * 0.7,
  );
  await page.mouse.down();
  await page.mouse.move(
    canvasBox!.x + 155,
    canvasBox!.y + canvasBox!.height * 0.7 - 35,
    { steps: 10 },
  );
  await page.mouse.up();
  await expect
    .poll(() => board.getAttribute("data-camera-target"))
    .not.toBe(oldTarget);
  await expect(board).toHaveAttribute("data-camera-polar", "0.785");
  const azimuth = await board.getAttribute("data-camera-azimuth");
  await page.mouse.move(
    canvasBox!.x + 75,
    canvasBox!.y + canvasBox!.height * 0.7,
  );
  await page.mouse.down({ button: "right" });
  await page.mouse.move(
    canvasBox!.x + 160,
    canvasBox!.y + canvasBox!.height * 0.7,
    { steps: 8 },
  );
  await page.mouse.up({ button: "right" });
  await expect
    .poll(() => board.getAttribute("data-camera-azimuth"))
    .not.toBe(azimuth);
  await expect(board).toHaveAttribute("data-camera-polar", "0.785");
  const oldDistance = Number(await board.getAttribute("data-camera-distance"));
  await page.getByTitle("Приблизить поле", { exact: true }).click();
  await expect
    .poll(async () => Number(await board.getAttribute("data-camera-distance")))
    .toBeLessThan(oldDistance - 0.5);
  await page.getByTitle("Границы областей", { exact: true }).click();
  await expect(board).toHaveAttribute("data-borders", "false");
  await page.getByTitle("Границы областей", { exact: true }).click();
  await page
    .getByTitle("Приблизить выбранную область", { exact: true })
    .click();
  await page.waitForTimeout(1600);
  await page.screenshot({ path: "test-results/game3d-winterfell.png" });
  await page.locator('[data-region3d="white-harbor"]').click();
  await expect(page.locator(".inspector h2")).toHaveText("Белая Гавань");
  const box = await canvas.boundingBox();
  await canvas.click({ position: { x: box!.width / 2, y: box!.height / 2 } });
  await expect(page.locator(".inspector h2")).toHaveText("Винтерфелл");
  await page
    .getByRole("button", { name: "Начать партию", exact: true })
    .click();
  await page.locator('[data-region3d="winterfell"]').click();
  await page.locator('.order-palette button[title="Поход"]').click();
  await expect(
    page.locator('[data-region3d="winterfell"] .game3d-order'),
  ).toHaveCount(1);
  const snapshot = () =>
    page.evaluate(async () => {
      const id = new URLSearchParams(location.search).get("room");
      return fetch(`/api/rooms/${id}`).then((r) => r.json());
    });
  const first = await snapshot();
  expect(first.orders.winterfell).toBe("march");
  expect(Object.keys(first.orders)).toEqual(["winterfell"]);
  await page
    .getByRole("button", { name: "Приказы готовы", exact: true })
    .click();
  await page.getByRole("button", { name: "К действиям", exact: true }).click();
  await expect
    .poll(async () => (await snapshot()).phase, { timeout: 30000 })
    .toBe("marches");
  if (await page.getByRole("dialog").count())
    await page.locator(".decision-next button").click();
  const panel = page.getByTestId("map-move-panel");
  await expect(panel).toHaveAttribute("data-selecting", "true", {
    timeout: 30000,
  });
  const destination = page.locator('[data-region3d="castle-black"]');
  await expect(destination).toHaveAttribute("data-eligible", "true");
  await panel.getByLabel("В поход: Пехота", { exact: true }).fill("0");
  await panel.getByLabel("В поход: Рыцари", { exact: true }).fill("0");
  await expect(destination).toHaveAttribute("data-eligible", "false");
  await expect(panel).toContainText("Нет допустимых целей");
  await panel.getByLabel("В поход: Пехота", { exact: true }).fill("1");
  await expect(destination).toHaveAttribute("data-eligible", "true");
  const beforePreview = await snapshot();
  // The earlier zoom/focus can put Castle Black behind the floating HUD.
  // Frame the legal targets just as a player would, instead of forcing a click
  // through the Houses toggle or the phase banner.
  await panel
    .getByRole("button", { name: "Показать цели", exact: true })
    .click();
  await destination.click();
  await expect(panel).toContainText("Чёрный замок");
  expect((await snapshot()).revision).toBe(beforePreview.revision);
  await expect(page.locator(".inspector h2")).toHaveText("Винтерфелл");
  await page.keyboard.press("Escape");
  await expect(panel).toHaveAttribute("data-selecting", "false");
  await panel
    .getByRole("button", { name: "Выбрать цель на карте", exact: true })
    .click();
  await expect(destination).toHaveAttribute("data-eligible", "true");
  await panel
    .getByRole("button", { name: "Показать цели", exact: true })
    .click();
  await destination.click();
  await panel
    .getByRole("button", { name: "Подтвердить поход", exact: true })
    .click();
  await expect
    .poll(async () => {
      const room = await snapshot();
      return (
        room.match.armies["castle-black"]?.reduce(
          (sum: number, u: { quantity: number }) => sum + u.quantity,
          0,
        ) || 0
      );
    })
    .toBe(1);
  const updated = await snapshot();
  await expect
    .poll(async () => Number(await board.getAttribute("data-revision")))
    .toBeGreaterThanOrEqual(updated.revision);
  await page.waitForTimeout(2000);
  await page.screenshot({ path: "test-results/game3d-march.png" });
  // Reload in the same browser retains both the room and renderer preference.
  await page.reload();
  await expect(page.getByTestId("game3d-board")).toHaveAttribute(
    "data-ready",
    "true",
    { timeout: 60000 },
  );
  expect(
    (await snapshot()).match.armies["castle-black"].length,
  ).toBeGreaterThan(0);
  expect((await snapshot()).match.armies.winterfell[0].unitType).toBe("knight");
  await expect(
    page.getByRole("button", { name: "2D", exact: true }),
  ).toHaveCount(0);
  expect(errors).toEqual([]);
});
