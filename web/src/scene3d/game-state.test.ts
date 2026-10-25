import { describe, expect, it } from "vitest";
import type { Room, Unit } from "../types";
import { formation, isGameManifest, reconcilePieces } from "./game-state";

const foot = (quantity: number, house = "stark"): Unit => ({
  quantity,
  house,
  unitType: "footman",
});
const room = (
  armies: Record<string, Unit[]>,
  events: Room["events"] = [],
  id = "room",
): Room => ({ id, events, match: { armies } }) as Room;

describe("3D presentation follows authoritative snapshots", () => {
  it("uses the selected lobby setup, not the six-house catalog army", () => {
    const lobby = {
      id: "room",
      events: [],
      setup: { armies: { winterfell: [foot(1)] } },
    } as unknown as Room;
    const state = reconcilePieces(undefined, lobby, {
      pyke: [foot(8, "greyjoy")],
    });
    expect(state.pieces).toHaveLength(1);
    expect(state.pieces[0].house).toBe("stark");
  });
  it("renders null armies and missing old event lists as empty collections", () => {
    const snapshot = JSON.parse(
      '{"id":"room","match":{"armies":{"winterfell":null}}}',
    ) as Room;
    const first = reconcilePieces(undefined, snapshot, {});
    expect(first.pieces).toEqual([]);
    expect(reconcilePieces(first, snapshot, {}).pieces).toEqual([]);
  });
  it("rejects partial or incompatible imports instead of opening a broken board", () => {
    expect(isGameManifest(undefined)).toBe(false);
    expect(isGameManifest({ version: 2 })).toBe(false);
    expect(isGameManifest({ version: 1, regions: {}, units: {} })).toBe(false);
  });
  it("uses lobby armies but does not repopulate an empty active match", () => {
    const initial = { winterfell: [foot(2)] };
    expect(
      reconcilePieces(
        undefined,
        { id: "room", events: [] } as unknown as Room,
        initial,
      ).pieces,
    ).toHaveLength(2);
    expect(reconcilePieces(undefined, room({}), initial).pieces).toHaveLength(
      0,
    );
  });
  it("keeps the identity of a moving figure and leaves remaining troops in place", () => {
    const initial = room({ winterfell: [foot(2)] });
    const before = reconcilePieces(undefined, initial, {});
    const after = reconcilePieces(
      before,
      room({ winterfell: [foot(1)], karhold: [foot(1)] }, [
        {
          id: 1,
          type: "march",
          house: "stark",
          from: "winterfell",
          to: "karhold",
          text: "",
        },
      ]),
      {},
    );
    expect(after.pieces.map((p) => p.id).sort()).toEqual(
      before.pieces.map((p) => p.id).sort(),
    );
    expect(after.pieces.find((p) => p.region === "winterfell")?.id).toBe(
      before.pieces[0].id,
    );
    expect(after.pieces.find((p) => p.region === "karhold")?.id).toBe(
      before.pieces[1].id,
    );
  });
  it("does not infer a march from unrelated casualty and recruitment changes", () => {
    const before = reconcilePieces(
      undefined,
      room({ winterfell: [foot(1)] }),
      {},
    );
    const after = reconcilePieces(before, room({ karhold: [foot(1)] }), {});
    expect(after.pieces[0].id).not.toBe(before.pieces[0].id);
  });
  it("does not replay old events on reload or reuse figures across rooms", () => {
    const events = [
      {
        id: 4,
        type: "march",
        house: "stark",
        from: "winterfell",
        to: "karhold",
        text: "",
      },
    ];
    const snapshot = room({ karhold: [foot(1)] }, events);
    const first = reconcilePieces(undefined, snapshot, {});
    const next = reconcilePieces(first, snapshot, {});
    expect(next).toEqual(first);
    const other = reconcilePieces(
      next,
      room({ winterfell: [foot(2)] }, [], "another"),
      {},
    );
    expect(other.eventId).toBe(0);
    expect(other.pieces.map((p) => p.region)).toEqual([
      "winterfell",
      "winterfell",
    ]);
  });
  it("preserves attacker identity through battle and victory without hiding defenders", () => {
    const before = reconcilePieces(
      undefined,
      room({ winterfell: [foot(1)], karhold: [foot(1, "lannister")] }),
      {},
    );
    const battle = {
      from: "winterfell",
      to: "karhold",
      attacker: "stark",
      defender: "lannister",
      army: [foot(1)],
    };
    const combat = {
      ...room({ karhold: [foot(1, "lannister")] }, [
        {
          id: 1,
          type: "march",
          house: "stark",
          from: "winterfell",
          to: "karhold",
          text: "",
        },
      ]),
      battle,
    } as Room;
    const during = reconcilePieces(before, combat, {});
    expect(during.pieces).toHaveLength(2);
    expect(during.pieces.find((p) => p.house === "stark")?.id).toBe(
      before.pieces[0].id,
    );
    expect(during.pieces.find((p) => p.house === "stark")?.battle).toBe(true);
    const after = reconcilePieces(
      during,
      room({ karhold: [foot(1)] }, combat.events),
      {},
    );
    expect(after.pieces).toHaveLength(1);
    expect(after.pieces[0].id).toBe(before.pieces[0].id);
    expect(after.pieces[0].battle).toBe(false);
  });
  it("does not mutate server armies and spreads individual figures", () => {
    const snapshot = room({ winterfell: [foot(4)] });
    const before = JSON.stringify(snapshot);
    const presentation = reconcilePieces(undefined, snapshot, {});
    expect(JSON.stringify(snapshot)).toBe(before);
    const points = presentation.pieces.map((piece) =>
      formation(piece, presentation.pieces, [1, 2]).join(","),
    );
    expect(new Set(points).size).toBe(4);
  });
});
