import { expect, test, type Page } from "@playwright/test";
import type { Room } from "../src/types";

declare global {
  interface Window {
    combatTestSocket?: WebSocket;
    combatTestConnections: number;
    combatTestRevision: number;
  }
}

type Fixture = {
  id: string;
  cookies: Record<string, { Name: string; Value: string }>;
};
const fixtures: Record<string, Fixture> = JSON.parse(
  process.env.WESTEROS_COMBAT_MATRIX_FIXTURES || "{}",
);
test.beforeEach(async ({ page }) => {
  test.skip(
    !process.env.WESTEROS_COMBAT_MATRIX_FIXTURES,
    "Run TestCombatMatrixBrowser against real Go HTTP/WS/SQLite",
  );
  await page.addInitScript(() => {
    localStorage.setItem("westeros-tour-v2", "true");
    localStorage.setItem("westeros-language", "en");
    localStorage.setItem("westeros-motion", "false");
    // Instrument the native connection only; messages still come from Go.
    const NativeSocket = window.WebSocket;
    window.combatTestConnections = 0;
    window.combatTestRevision = -1;
    window.WebSocket = class extends NativeSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        super(url, protocols);
        window.combatTestSocket = this;
        window.combatTestConnections++;
        this.addEventListener("message", (event) => {
          const snapshot = JSON.parse(event.data);
          if (typeof snapshot.revision === "number") {
            window.combatTestRevision = Math.max(
              window.combatTestRevision,
              snapshot.revision,
            );
          }
        });
      }
    };
  });
});
async function state(page: Page, f: Fixture, house?: string): Promise<Room> {
  const cookie = house ? f.cookies[house] : undefined;
  const response = await page.request.get(`/api/rooms/${f.id}`, {
    headers: cookie ? { Cookie: `${cookie.Name}=${cookie.Value}` } : {},
  });
  expect(response.ok()).toBe(true);
  return response.json();
}
async function command(
  page: Page,
  f: Fixture,
  house: string,
  body: Record<string, unknown>,
) {
  const cookie = f.cookies[house];
  const response = await page.request.post(`/api/rooms/${f.id}/commands`, {
    headers: { Cookie: `${cookie.Name}=${cookie.Value}` },
    data: { ...body, revision: (await state(page, f, house)).revision },
  });
  expect(response.ok(), await response.text()).toBe(true);
  const next: Room = await response.json();
  // External API commands do not update React directly. Wait for their real
  // WebSocket projection before issuing the next command or inspecting the UI.
  await expect
    .poll(() => page.evaluate(() => window.combatTestRevision), {
      timeout: 20000,
      message: `The browser receives server revision ${next.revision}`,
    })
    .toBeGreaterThanOrEqual(next.revision);
}
async function open(page: Page, baseURL: string, f: Fixture, house = "tyrell") {
  const cookie = f.cookies[house];
  await page
    .context()
    .addCookies([{ name: cookie.Name, value: cookie.Value, url: baseURL }]);
  await page.goto(`/?room=${f.id}`);
  await expect
    .poll(() => page.evaluate(() => window.combatTestSocket?.readyState), {
      timeout: 20000,
      message:
        "The real game WebSocket is connected before test commands start",
    })
    .toBe(1);
}
async function readIntro(page: Page) {
  const intro = page.locator(".combat-intro");
  await expect(intro).toBeVisible({ timeout: 20000 });
  await expect(page.getByRole("dialog")).toHaveCount(1);
  await expect(intro.locator('[data-kind="house_card"]')).toHaveCount(0);
  await intro
    .getByRole("button", { name: "Skip movement", exact: true })
    .click();
}
async function checkReport(page: Page, f: Fixture) {
  const report = page.locator(".combat-report");
  await expect(report).toBeVisible({ timeout: 20000 });
  await expect(report.locator(".combat-card-back")).toHaveCount(2);
  const record = (await state(page, f)).match!.combats![0];
  const result = record.result!;
  expect(result.final).toBeDefined();
  for (const [side, total] of [
    [result.strength!.attacker, result.attack],
    [result.strength!.defender, result.defense],
  ] as const) {
    expect(side.lines.reduce((sum, line) => sum + line.value, 0)).toBe(total);
    expect(side.total).toBe(total);
  }
  await report
    .getByRole("button", { name: "Reveal cards", exact: true })
    .click();
  await report
    .getByRole("button", { name: "Calculate strength", exact: true })
    .click();
  await expect(report.getByTestId("strength-total")).toHaveText([
    String(result.attack),
    String(result.defense),
  ]);
  const before = (await state(page, f)).match!.combats;
  await report
    .getByRole("button", {
      name: "Watch the outcome on the board",
      exact: true,
    })
    .click();
  const playback = page.locator(".combat-playback-controls");
  await expect(playback).toHaveAttribute("data-state", "complete");
  const board = page.getByTestId("game3d-board");
  if (await board.count()) {
    await expect(board).toHaveAttribute("data-ready", "true", {
      timeout: 60000,
    });
    const expected = Object.values(result.final!.armies)
      .flat()
      .reduce((sum, unit) => sum + unit.quantity, 0);
    await expect(board).toHaveAttribute(
      "data-playback-pieces",
      String(expected),
    );
    for (const [region, army] of Object.entries(result.final!.armies)) {
      for (const house of [record.attacker, record.defender]) {
        const amount = army
          .filter((unit) => unit.house === house)
          .reduce((sum, unit) => sum + unit.quantity, 0);
        const badge = page.locator(
          `[data-region3d="${region}"] [data-unit-house="${house}"]`,
        );
        if (amount) await expect(badge).toHaveText(String(amount));
        else await expect(badge).toHaveCount(0);
      }
    }
  }
  expect((await state(page, f)).match!.combats).toEqual(before);
  await playback
    .getByRole("button", { name: "Return to battle", exact: true })
    .click();
  await expect(report.locator(".combat-victory-banner")).toHaveCount(1);
  return record;
}

for (const mode of [
  "catelyn",
  "plain",
  "arianne",
  "renly",
  "trapped",
  "skull",
]) {
  test(`complex combat final snapshot and exact breakdown: ${mode}`, async ({
    page,
    baseURL,
  }) => {
    test.setTimeout(90000);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    if (mode === "renly")
      await page.setViewportSize({ width: 820, height: 1180 });
    const f = fixtures[mode];
    await open(page, baseURL!, f);
    await readIntro(page);
    const record = await checkReport(page, f);
    const result = record.result!;
    if (mode === "catelyn") {
      expect(
        result.strength!.attacker.lines.some(
          (line) =>
            line.kind === "unit_effect" && line.card === "ser-kevan-lannister",
        ),
      ).toBe(true);
      expect(
        result.strength!.defender.lines.some(
          (line) =>
            line.kind === "card_effect" && line.card === "catelyn-stark",
        ),
      ).toBe(true);
    }
    if (mode === "plain") {
      expect(record.initial.castle).toBe(false);
      expect(
        record.initial.attacker.lines.find(
          (line) => line.unit === "siege-engine",
        )!.value,
      ).toBe(0);
    }
    if (mode === "arianne") {
      expect(result.winner).toBe(record.attacker);
      expect(result.returnTo).toBe(record.from);
      expect(result.final!.control[record.to]).not.toBe(record.attacker);
      await expect(page.locator(".combat-casualties")).toContainText(
        "winning units return",
      );
    }
    if (mode === "renly") {
      expect(
        result
          .final!.armies[record.to].filter(
            (u) => u.house === record.attacker && u.unitType === "knight",
          )
          .reduce((s, u) => s + u.quantity, 0),
      ).toBe(2);
    }
    if (mode === "trapped") {
      expect(result.retreatHouse).toBeFalsy();
      expect(
        result.losses![record.defender].reduce((s, u) => s + u.quantity, 0),
      ).toBe(2);
    }
    if (mode === "skull") {
      expect(result.winner).toBe(record.attacker);
      expect(result.final!.armies[record.to]).toHaveLength(0);
      expect(result.final!.control[record.to]).not.toBe(record.attacker);
    }
    await page.screenshot({
      path: test.info().outputPath(`${mode}-result.png`),
    });
    expect(errors).toEqual([]);
  });
}

for (const mode of ["tides-blade", "tides-redraw"]) {
  test(`private Tides choice, reload and exact public report: ${mode}`, async ({
    page,
    baseURL,
  }) => {
    test.setTimeout(90000);
    const f = fixtures[mode];
    await open(page, baseURL!, f, "stark");
    await readIntro(page);
    await command(page, f, "stark", { type: "card", card: "greatjon-umber" });
    await command(page, f, "lannister", {
      type: "card",
      card: "ser-kevan-lannister",
    });
    const dialog = page.getByTestId("card-effect-choice");
    await expect(dialog).toBeVisible({ timeout: 20000 });
    const pending = await state(page, f);
    expect(Object.keys(pending.battle!.modifiers!)).toEqual(["stark"]);
    const foreign = await state(page, f, "tyrell");
    expect(Object.keys(foreign.battle!.modifiers || {})).toHaveLength(0);
    expect(foreign.match!.combats![0].result).toBeUndefined();
    expect(JSON.stringify(foreign)).not.toContain('"deck"');
    await expect(dialog.locator('[data-card-id="hidden"]')).toHaveCount(1);
    // Room snapshots may arrive by HTTP before the native WS handshake has
    // completed. Close a confirmed OPEN socket, not a still-connecting one.
    await expect
      .poll(() => page.evaluate(() => window.combatTestSocket?.readyState), {
        timeout: 20000,
        message:
          "Initial game WebSocket is open before simulating a disconnect",
      })
      .toBe(1);
    const connections = await page.evaluate(async () => {
      const socket = window.combatTestSocket!;
      const count = window.combatTestConnections;
      await new Promise<void>((resolve) => {
        socket.addEventListener("close", () => resolve(), { once: true });
        socket.close(1000, "test reconnect");
      });
      return count;
    });
    await expect
      .poll(
        () =>
          page.evaluate(
            (count) =>
              window.combatTestConnections > count &&
              window.combatTestSocket?.readyState === 1,
            connections,
          ),
        {
          timeout: 20000,
          message: "A new native game WebSocket opens after disconnect",
        },
      )
      .toBe(true);
    await expect(dialog).toBeVisible();
    expect((await state(page, f)).battle!.modifiers).toEqual(
      pending.battle!.modifiers,
    );
    await page.reload();
    await expect(dialog).toBeVisible({ timeout: 20000 });
    expect((await state(page, f)).battle!.modifiers).toEqual(
      pending.battle!.modifiers,
    );
    await dialog
      .getByRole("button", {
        name:
          mode === "tides-redraw"
            ? "Replace card · spend Blade"
            : "Keep without using Blade",
        exact: true,
      })
      .click();
    if (mode === "tides-blade") {
      await expect(dialog.locator('[data-card-id="hidden"]')).toHaveCount(0);
      await dialog
        .getByRole("button", { name: "Add +1 · spend Blade", exact: true })
        .click();
    }
    let room = await state(page, f);
    if (room.phase === "retreat") {
      const house = room.battle!.retreatChooser || room.battle!.loser!;
      room = await state(page, f, house);
      await command(page, f, house, {
        type: "retreat",
        target: room.battle!.retreats[0],
      });
    }
    const record = await checkReport(page, f);
    expect(Object.keys(record.result!.modifiers!).sort()).toEqual([
      "lannister",
      "stark",
    ]);
    expect(
      record.result!.strength!.attacker.lines.some(
        (line) => line.kind === "tides",
      ),
    ).toBe(true);
    if (mode === "tides-blade") expect(record.result!.bladeBonus).toBe("stark");
    else expect(record.result!.bladeRedraw).toBe("stark");
  });
}

for (const mode of ["bot-human", "bot-bot"]) {
  test(`real scheduled attack and queued presentation: ${mode}`, async ({
    page,
    baseURL,
  }) => {
    test.setTimeout(120000);
    const f = fixtures[mode];
    const frames: Room[] = [];
    page.on("websocket", (socket) =>
      socket.on("framereceived", (event) => {
        try {
          frames.push(JSON.parse(String(event.payload)));
        } catch {
          /* Non-JSON frames are not state snapshots. */
        }
      }),
    );
    await open(
      page,
      baseURL!,
      f,
      mode === "bot-human" ? "lannister" : "tyrell",
    );
    await expect
      .poll(() => page.evaluate(() => window.combatTestSocket?.readyState), {
        timeout: 20000,
      })
      .toBe(1);
    await expect
      .poll(() => frames.length, { timeout: 20000 })
      .toBeGreaterThan(0);
    if (mode === "bot-bot") {
      await page
        .getByRole("button", { name: "Event history", exact: true })
        .click({ timeout: 20000 });
      await expect(page.locator(".events-modal")).toBeVisible();
    }
    await command(page, f, "tyrell", { type: "pass", region: "highgarden" });
    await expect
      .poll(async () => (await state(page, f)).match?.combats?.length, {
        timeout: 20000,
      })
      .toBe(1);
    if (mode === "bot-bot") {
      await expect
        .poll(
          async () => !!(await state(page, f)).match!.combats![0].result?.final,
          { timeout: 20000 },
        )
        .toBe(true);
      await expect(page.locator(".events-modal")).toBeVisible();
      await expect(page.locator(".combat-intro")).toHaveCount(0);
      await expect(page.getByRole("dialog")).toHaveCount(1);
      await page
        .locator(".events-modal")
        .getByRole("button", { name: "Close", exact: true })
        .click();
    } else {
      await expect
        .poll(async () => !!(await state(page, f)).battle?.chosen.stark, {
          timeout: 20000,
        })
        .toBe(true);
      const privateView = await state(page, f);
      expect(privateView.battle!.cards).toBeUndefined();
      expect(privateView.match!.combats![0].result).toBeUndefined();
      await expect
        .poll(
          () =>
            frames.filter(
              (frame) =>
                frame.battle?.chosen.stark && !frame.battle?.chosen.lannister,
            ).length,
        )
        .toBeGreaterThan(0);
      expect(
        frames
          .filter(
            (frame) =>
              frame.battle?.chosen.stark && !frame.battle?.chosen.lannister,
          )
          .every(
            (frame) =>
              !frame.battle?.cards && !frame.match?.combats?.[0].result,
          ),
      ).toBe(true);
    }
    await readIntro(page);
    if (mode === "bot-human") {
      await page
        .locator(".combat-card-choice .battle-hand button")
        .filter({ hasText: "Ser Kevan Lannister" })
        .click();
    }
    await expect
      .poll(
        async () => !!(await state(page, f)).match!.combats![0].result?.final,
        { timeout: 20000 },
      )
      .toBe(true);
    const record = await checkReport(page, f);
    expect(record.attackers.reduce((sum, unit) => sum + unit.quantity, 0)).toBe(
      4,
    );
    expect(record.remaining.reduce((sum, unit) => sum + unit.quantity, 0)).toBe(
      1,
    );
    expect(record.result!.attackerCard).toBe("eddard-stark");
    expect(frames.some((frame) => frame.match?.combats?.length)).toBe(true);
  });
}
