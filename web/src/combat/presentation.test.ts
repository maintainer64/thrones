import { describe, expect, it } from "vitest";
import type { CombatRecord, Room } from "../types";
import { hasCombatDecision, nextCombatStep, stepID } from "./presentation";

const record = (id: number, finished = true) =>
  ({
    id,
    round: 1,
    attacker: "stark",
    defender: "greyjoy",
    result: finished ? { id } : undefined,
  }) as CombatRecord;
const room = (records: CombatRecord[]) =>
  ({
    id: "room",
    phase: "westeros",
    yourHouse: "stark",
    match: { combats: records },
  }) as Room;

describe("persisted combat presentation", () => {
  it("recovers every battle when one WS snapshot skips multiple bot battles", () => {
    const snapshot = room([record(1), record(2), record(3)]);
    const read: string[] = [];
    for (const expected of [
      "1:intro",
      "1:result",
      "2:intro",
      "2:result",
      "3:intro",
      "3:result",
    ]) {
      const step = nextCombatStep(snapshot, read)!;
      expect(stepID(step)).toBe(expected);
      read.push(stepID(step));
    }
    expect(nextCombatStep(snapshot, read)).toBeUndefined();
  });
  it("shows the result once after the intro, even across rounds/reload and effect changes", () => {
    const snapshot = room([record(5)]);
    expect(stepID(nextCombatStep(snapshot, ["5:intro"])!)).toBe("5:result");
    expect(
      nextCombatStep({ ...snapshot, round: 2 }, ["5:intro", "5:result"]),
    ).toBeUndefined();
    const changed = structuredClone(snapshot);
    changed.match!.combats![0].result!.effects = ["Translated effect"];
    expect(nextCombatStep(changed, ["5:intro", "5:result"])).toBeUndefined();
  });
  it("never invents a result for an unresolved battle", () => {
    expect(
      nextCombatStep(room([record(1, false)]), ["1:intro"]),
    ).toBeUndefined();
  });
  it("gives access to one's live choice ahead of old bot reports", () => {
    const snapshot = {
      ...room([record(1), record(2, false)]),
      phase: "combat",
      battle: { id: 2, attacker: "stark", defender: "greyjoy" },
    } as Room;
    expect(hasCombatDecision(snapshot)).toBe(true);
    expect(stepID(nextCombatStep(snapshot, [])!)).toBe("2:intro");
    expect(nextCombatStep(snapshot, ["2:intro"])).toBeUndefined();
    snapshot.battle!.yourCard = "eddard-stark";
    expect(stepID(nextCombatStep(snapshot, ["2:intro"])!)).toBe("1:intro");
  });
  it("does not block an ability or map retreat with older reports", () => {
    for (const phase of ["card_effect", "retreat"]) {
      const snapshot = {
        ...room([record(1), record(2)]),
        phase,
        battle: { id: 2, choice: { house: "stark" }, retreatChooser: "stark" },
      } as Room;
      expect(nextCombatStep(snapshot, ["2:intro"])).toBeUndefined();
    }
  });
});
