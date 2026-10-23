import { describe, it, expect } from "vitest";
import { updateDecision } from "./decision-flow";
import type { DecisionState } from "./decision-flow";
import type { Room } from "../types";

const overview = {
  id: "room",
  round: 2,
  revision: 1,
  phase: "westeros",
  westeros: { run: { round: 2 }, current: { round: 2 } },
} as Room;
const muster = {
  ...overview,
  revision: 2,
  phase: "mustering",
  match: { activeHouse: "stark" },
} as Room;
describe("one stable decision window", () => {
  it("lets queued combats finish before the next round's event without dismissing it", () => {
    const state = updateDecision({}, { type: "observe", room: overview });
    const nextRound = { ...overview, round: 3, revision: 4 };
    const next = updateDecision(state, {
      type: "next",
      room: nextRound,
      blocked: true,
    });
    expect(next.active).toBeUndefined();
    expect(
      updateDecision(next, { type: "observe", room: nextRound, blocked: true })
        .active,
    ).toBeUndefined();
    expect(
      updateDecision(next, { type: "observe", room: nextRound, blocked: false })
        .active?.room.round,
    ).toBe(3);
  });
  it("never replaces an open event with an automatic mustering or auction window", () => {
    let state: DecisionState = updateDecision(
      {},
      { type: "observe", room: overview },
    );
    state = updateDecision(state, { type: "observe", room: muster });
    expect(state.active?.kind).toBe("events");
    expect(state.active?.room).toBe(overview);
    state = updateDecision(state, { type: "next", room: muster });
    expect(state.active?.kind).toBe("provision");
    expect(state.active?.room).toBe(muster);
  });
  it("updates choices inside the same step but pins the previous actor while bots advance", () => {
    const active = updateDecision({}, { type: "observe", room: muster });
    const changed = { ...muster, revision: 3 };
    const updated = updateDecision(active, { type: "observe", room: changed });
    expect(updated.active?.room).toBe(changed);
    const other = {
      ...changed,
      revision: 4,
      match: { activeHouse: "greyjoy" },
    } as Room;
    expect(
      updateDecision(updated, { type: "observe", room: other }).active?.room,
    ).toBe(changed);
  });
  it("explicit confirmation may advance, closing does not immediately reopen the same step", () => {
    const state = updateDecision({}, { type: "observe", room: overview });
    expect(
      updateDecision(state, { type: "commit", room: muster }).active?.kind,
    ).toBe("provision");
    const closed = updateDecision(state, { type: "close", room: muster });
    expect(
      updateDecision(closed, { type: "observe", room: muster }).active,
    ).toBeUndefined();
    expect(
      updateDecision({}, { type: "observe", room: muster, blocked: true })
        .active,
    ).toBeUndefined();
  });
  it("does not retain private snapshots when entering another room or leaving", () => {
    const state = updateDecision({}, { type: "observe", room: muster });
    expect(updateDecision(state, { type: "observe", room: undefined })).toEqual(
      {},
    );
    expect(
      updateDecision(state, {
        type: "observe",
        room: { ...overview, id: "other" },
      }).active?.room.id,
    ).toBe("other");
  });
});
