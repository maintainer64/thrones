import { test, expect } from "@playwright/test";

for (const failedPreview of [false, true]) {
  test(`raid has one reopenable sidebar and can be passed (${failedPreview ? "preview error" : "no legal targets"})`, async ({
    page,
  }) => {
    test.setTimeout(90000);
    await page.addInitScript(() =>
      localStorage.setItem("westeros-tour-v2", "true"),
    );
    await page.goto("/");
    await page
      .getByLabel("Имя игрока", { exact: true })
      .fill("Raid regression");
    await page.getByRole("button", { name: "Собрать малый совет" }).click();
    await expect(page.getByTestId("game3d-board")).toHaveAttribute(
      "data-ready",
      "true",
      { timeout: 60000 },
    );
    await page.getByRole("button", { name: "Начать партию" }).click();
    const snapshot = () =>
      page.evaluate(async () => {
        const id = new URLSearchParams(location.search).get("room");
        return fetch(`/api/rooms/${id}`, {
          headers: { "Accept-Language": "ru" },
        }).then((response) => response.json());
      });
    const command = (body: Record<string, unknown>) =>
      page.evaluate(async (body) => {
        const id = new URLSearchParams(location.search).get("room");
        for (let attempt = 0; attempt < 15; attempt++) {
          const room = await fetch(`/api/rooms/${id}`).then((response) =>
            response.json(),
          );
          const response = await fetch(`/api/rooms/${id}/commands`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ ...body, revision: room.revision }),
          });
          if (response.status === 409) {
            await new Promise((resolve) => setTimeout(resolve, 100));
            continue;
          }
          if (!response.ok) throw Error(await response.text());
          return;
        }
        throw Error(
          "Could not obtain a stable room revision while bots were planning",
        );
      }, body);
    await command({ type: "order", region: "winterfell", order: "raid-1" });
    await command({ type: "order", region: "white-harbor", order: "march" });
    await command({ type: "ready" });
    await expect
      .poll(async () => (await snapshot()).phase, { timeout: 30000 })
      .toBe("orders_revealed");
    if (failedPreview)
      await page.route("**/preview", (route) =>
        route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ error: "Не удалось получить цели" }),
        }),
      );
    await command({ type: "continue" });
    await expect
      .poll(async () => (await snapshot()).phase, { timeout: 30000 })
      .toBe("raids");
    if (await page.getByRole("dialog").count())
      await page.locator(".decision-next button").click();
    const panel = page.getByTestId("map-move-panel");
    await expect(panel).toHaveAttribute("data-selecting", "true", {
      timeout: 30000,
    });
    await expect(panel).toContainText("НАБЕГ");
    const skip = panel.getByRole("button", {
      name: "Снять приказ без действия",
      exact: true,
    });
    if (failedPreview)
      await expect(panel).toContainText("Не удалось получить цели");
    else {
      await expect(panel).toContainText("Рядом нет чужих приказов");
      await expect(
        panel.getByRole("button", { name: "Подтвердить набег", exact: true }),
      ).toBeDisabled();
      await expect(skip).toBeEnabled();
      await expect(panel).not.toContainText("Ищем допустимые цели");
    }
    await page
      .getByRole("button", { name: "Панель выбранной области", exact: true })
      .click();
    await expect(page.locator(".inspector")).not.toBeVisible();
    await expect(page.locator(".move-dock")).toHaveCount(0);
    await page
      .getByRole("button", { name: "Открыть панель действий", exact: true })
      .click();
    await expect(panel).toHaveCount(1);
    const before = await snapshot();
    await expect(skip).toBeEnabled();
    await expect(skip).toBeInViewport();
    await skip.click();
    await expect
      .poll(async () => (await snapshot()).orders.winterfell)
      .toBeUndefined();
    const after = await snapshot();
    expect(after.revision).toBeGreaterThan(before.revision);
    expect(after.match.armies.winterfell).toEqual(
      before.match.armies.winterfell,
    );
    expect(
      after.log.some((line: string) =>
        line.includes("приказ снят без выполнения"),
      ),
    ).toBe(true);
  });
}
