import { test, expect, type APIRequestContext } from "@playwright/test";

test("Russian card rules and a real Aeron replacement survive page reload", async ({
  page,
  browser,
  baseURL,
}) => {
  test.setTimeout(90000);
  await page.addInitScript(() =>
    localStorage.setItem("westeros-tour-v2", "true"),
  );
  // Keep this a card/HTTP/WS test; map rendering has its own real-browser suite.
  await page.route("**/local-assets/preview3d/game-manifest.json", (route) =>
    route.fulfill({ status: 404, body: "card-only test" }),
  );
  const created = await page.request
    .post("/api/rooms", { data: { name: "Card abilities" } })
    .then((r) => r.json());
  const path = `/api/rooms/${created.id}`;
  const snapshot = () => page.request.get(path).then((r) => r.json());
  const command = async (
    client: APIRequestContext,
    data: Record<string, unknown>,
  ) => {
    const current = await client.get(path).then((r) => r.json());
    const response = await client.post(`${path}/commands`, {
      data: { ...data, revision: current.revision },
    });
    expect(response.ok(), await response.text()).toBe(true);
    return response.json();
  };
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
    await page.getByRole("button", { name: /^Робб Старк · сила 3/ }).click();
    await expect(page.locator("#card-title")).toHaveText("Робб Старк");
    await expect(page.getByLabel("Эффект карты")).toContainText(
      "отступление противника",
    );
    await expect(page.getByLabel("Эффект карты")).toContainText(
      "Эффект исполняется сервером",
    );
    await page
      .getByRole("button", { name: "Закрыть карту", exact: true })
      .click();
    await command(page.request, { type: "start" });
    const greyjoy = guests.find((g) => g.house === "greyjoy")!.context;
    await command(page.request, {
      type: "order",
      region: "winterfell",
      order: "march",
    });
    await command(greyjoy.request, {
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
    await command(greyjoy.request, {
      type: "march",
      region: "greywater-watch",
      target: "moat-cailin",
      units: { footman: 1 },
    });
    await command(page.request, { type: "card", card: "eddard-stark" });
    await command(greyjoy.request, { type: "card", card: "aeron-damphair" });
    expect((await snapshot()).phase).toBe("card_effect");
    const opponent = await greyjoy.newPage();
    await opponent.addInitScript(() =>
      localStorage.setItem("westeros-tour-v2", "true"),
    );
    await opponent.route(
      "**/local-assets/preview3d/game-manifest.json",
      (route) => route.fulfill({ status: 404, body: "card-only test" }),
    );
    const errors: string[] = [];
    opponent.on("pageerror", (e) => errors.push(e.message));
    await opponent.goto(`/?room=${created.id}`);
    const dialog = opponent.getByTestId("card-effect-choice");
    await expect(dialog).toContainText("Эйрон Мокроголовый");
    await opponent.reload();
    await expect(dialog).toBeVisible();
    const before = await snapshot();
    await dialog
      .getByRole("button", { name: "Бейлон Грейджой", exact: true })
      .click();
    await expect(dialog).toHaveCount(0);
    await expect.poll(async () => (await snapshot()).phase).toBe("retreat");
    const after = await snapshot();
    expect(after.match.power.greyjoy).toBe(before.match.power.greyjoy - 2);
    expect(after.match.discard.greyjoy).toContain("aeron-damphair");
    expect(after.match.lastCombat).toMatchObject({
      attackerCard: "balon-greyjoy",
      defenderCard: "eddard-stark",
      attack: 3,
      defense: 1,
      winner: "greyjoy",
    });
    expect(after.match.lastCombat.effects.join(" ")).toContain(
      "Эйрон Мокроголовый",
    );
    expect(errors).toEqual([]);
  } finally {
    await Promise.all(guests.map((g) => g.context.close()));
  }
});
