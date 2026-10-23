import { describe, expect, it } from "vitest";
import { CombatLineKind } from "../enums";
import type { CombatRecord, Room } from "../types";
import {
  ReplayPhase,
  advanceReplay,
  replayActors,
  replayPieces,
  replayPose,
  replayRoom,
  strengthAtStep,
  strengthStepCount,
} from "./animation";

const result = (
  changes: Partial<NonNullable<CombatRecord["result"]>>,
): NonNullable<CombatRecord["result"]> => ({
  attacker: "stark",
  defender: "lannister",
  attackerCard: "eddard-stark",
  defenderCard: "tywin-lannister",
  attack: 6,
  defense: 10,
  winner: "lannister",
  region: "white-harbor",
  ...changes,
});

const record = (): CombatRecord => ({
  id: 1,
  round: 1,
  from: "winterfell",
  to: "white-harbor",
  attacker: "stark",
  defender: "lannister",
  attackers: [{ house: "stark", unitType: "footman", quantity: 2 }],
  defenders: [{ house: "lannister", unitType: "knight", quantity: 2 }],
  remaining: [{ house: "stark", unitType: "footman", quantity: 1 }],
  initial: {
    revealed: false,
    castle: true,
    attacker: {
      house: "stark",
      total: 2,
      lines: [
        {
          kind: CombatLineKind.Unit,
          unit: "footman",
          count: 2,
          each: 1,
          value: 2,
        },
        { kind: CombatLineKind.March, value: -1 },
        {
          kind: CombatLineKind.SupportUnit,
          region: "widows-watch",
          unit: "footman",
          count: 1,
          each: 1,
          value: 1,
        },
      ],
    },
    defender: {
      house: "lannister",
      total: 6,
      lines: [
        {
          kind: CombatLineKind.Unit,
          unit: "knight",
          count: 2,
          each: 2,
          value: 4,
        },
        { kind: CombatLineKind.Garrison, value: 2 },
      ],
    },
  },
});

describe("immutable historical combat animation", () => {
  it("uses the recorded final controller even when a skull kills the winning army", () => {
    const value = record();
    value.result = result({
      winner: "stark",
      final: {
        armies: { winterfell: value.remaining, "white-harbor": [] },
        control: { winterfell: "stark", "white-harbor": "" },
        garrisons: {},
      },
    });
    const live = {
      id: "room",
      match: { armies: {}, control: { "white-harbor": "lannister" } },
    } as unknown as Room;
    const end = replayRoom(live, {
      record: value,
      phase: ReplayPhase.Losses,
      serial: 1,
      playing: false,
      complete: true,
    });
    expect(end.match!.control["white-harbor"]).toBe("");
    expect(replayPieces(value, ReplayPhase.Losses, true)).toHaveLength(1);
    expect(live.match!.control["white-harbor"]).toBe("lannister");
  });
  it("shows recorded upgrades instead of recreating the final army from casualties alone", () => {
    const value = record();
    value.result = result({
      final: {
        armies: {
          "white-harbor": [{ house: "stark", unitType: "knight", quantity: 1 }],
        },
        control: { "white-harbor": "stark" },
        garrisons: {},
      },
    });
    expect(
      replayPieces(value, ReplayPhase.Losses, true).map((p) => p.type),
    ).toEqual(["knight"]);
    expect(
      replayPieces(value, ReplayPhase.Approach, true).filter(
        (p) => p.house === "stark" && p.type === "footman",
      ),
    ).toHaveLength(4);
  });
  it("animates the winner returning to its origin when that destination is recorded", () => {
    const value = record();
    value.result = result({ winner: "stark", returnTo: "winterfell" });
    const attackers = replayActors(value).filter((a) => a.role === "attacker");
    expect(attackers.every((a) => a.retreatTo === "winterfell")).toBe(true);
    expect(replayPose(attackers[0], ReplayPhase.Losses, 1).retreat).toBe(1);
  });
  it("finishes in elapsed time rather than stretching the scene on slow GPUs", () => {
    let progress = 0;
    for (let frame = 0; frame < 3; frame++)
      progress = advanceReplay(progress, 1.5, ReplayPhase.Losses);
    expect(progress).toBe(1);
    expect(advanceReplay(0.5, 0, ReplayPhase.Approach)).toBe(0.5);
    expect(advanceReplay(0.5, -1, ReplayPhase.Approach)).toBe(0.5);
    expect(advanceReplay(0, 4, ReplayPhase.Approach)).toBe(1);
  });
  it("moves only sent attackers; keeps defenders, support and remaining units in place", () => {
    const value = record();
    const actors = replayActors(value);
    expect(actors).toHaveLength(6);
    for (const actor of actors) {
      expect(actor.lost).toBe(false);
      expect(actor.retreatTo).toBeUndefined();
      expect(replayPose(actor, ReplayPhase.Approach, -1).travel).toBe(0);
      expect(replayPose(actor, ReplayPhase.Approach, 2).travel).toBe(
        actor.role === "attacker" ? 1 : 0,
      );
    }
    expect(
      replayPieces(value, ReplayPhase.Approach, false).filter(
        (p) => p.region === "winterfell",
      ),
    ).toHaveLength(3);
    expect(
      replayPieces(value, ReplayPhase.Approach, true).filter(
        (p) => p.region === "winterfell",
      ),
    ).toHaveLength(1);
    expect(
      replayPieces(value, ReplayPhase.Approach, true).filter(
        (p) => p.region === "white-harbor",
      ),
    ).toHaveLength(4);
  });
  it("animates only recorded participant casualties and saved retreats, not supporting units", () => {
    const value = record();
    value.result = result({
      losses: { stark: [{ house: "stark", unitType: "footman", quantity: 1 }] },
      retreatHouse: "stark",
      retreatTo: "winterfell",
    });
    const before = structuredClone(value);
    const actors = replayActors(value);
    expect(actors.filter((a) => a.lost)).toHaveLength(1);
    expect(actors.filter((a) => a.retreatTo)).toHaveLength(1);
    expect(
      actors
        .filter((a) => a.role === "remaining" || a.role === "support")
        .every((a) => !a.lost && !a.retreatTo),
    ).toBe(true);
    const dead = actors.find((a) => a.lost)!;
    expect(replayPose(dead, ReplayPhase.Approach, 1).fade).toBe(0);
    expect(replayPose(dead, ReplayPhase.Losses, 1).fade).toBe(1);
    const survivor = actors.find((a) => a.retreatTo)!;
    expect(replayPose(survivor, ReplayPhase.Losses, 0.5).retreat).toBe(0);
    expect(replayPose(survivor, ReplayPhase.Losses, 1).retreat).toBe(1);
    const pieces = replayPieces(value, ReplayPhase.Losses, true);
    expect(pieces).toHaveLength(5);
    expect(pieces.filter((p) => p.region === "winterfell")).toHaveLength(2);
    expect(pieces.filter((p) => p.region === "widows-watch")).toHaveLength(1);
    expect(value).toEqual(before);
  });
  it("does not predict a retreat destination while the live player is choosing", () => {
    const value = record();
    value.result = result({ retreatHouse: "stark" });
    expect(replayActors(value).every((a) => !a.retreatTo)).toBe(true);
    expect(
      replayPieces(value, ReplayPhase.Losses, true).filter(
        (p) => p.region === "white-harbor",
      ),
    ).toHaveLength(4);
  });
  it("allocates a loss once even when identical units are stored in multiple stacks", () => {
    const value = record();
    value.attackers = [
      value.attackers[0],
      { ...value.attackers[0], quantity: 1 },
    ];
    value.result = result({
      losses: { stark: [{ house: "stark", unitType: "footman", quantity: 1 }] },
    });
    expect(replayActors(value).filter((a) => a.lost)).toHaveLength(1);
  });
  it("builds a renderer-only facade without mixing in armies, orders or forts from the live round", () => {
    const value = record();
    value.result = result({
      winner: "stark",
      losses: { lannister: value.defenders },
    });
    const room = {
      id: "room",
      round: 8,
      battle: { from: "other" },
      events: [{}],
      orders: { winterfell: "march" },
      match: {
        armies: {
          "kings-landing": [
            { house: "baratheon", unitType: "footman", quantity: 10 },
          ],
        },
        control: { "kings-landing": "baratheon" },
      },
    } as unknown as Room;
    const before = structuredClone(room);
    const playback = {
      record: value,
      phase: ReplayPhase.Losses,
      serial: 1,
      playing: true,
      complete: false,
    };
    const facade = replayRoom(room, playback);
    expect(facade.round).toBe(1);
    expect(facade.match!.armies["kings-landing"]).toBeUndefined();
    expect(facade.battle).toBeUndefined();
    expect(facade.orders).toEqual({});
    expect(facade.fortifications?.[value.to]?.strength).toBe(2);
    const end = replayRoom(room, { ...playback, complete: true });
    expect(end.fortifications).toEqual({});
    expect(end.match!.control[value.to]).toBe("stark");
    expect(room).toEqual(before);
  });
  it("adds server contributions in order, including negative values, and reaches exact totals", () => {
    const value = record().initial;
    expect(strengthStepCount(value)).toBe(3);
    expect(strengthAtStep(value, 0).attacker.total).toBe(0);
    expect(strengthAtStep(value, 1).attacker.total).toBe(2);
    expect(strengthAtStep(value, 2).attacker.total).toBe(1);
    const last = strengthAtStep(value, 99);
    expect(last.attacker.total).toBe(value.attacker.total);
    expect(last.defender.total).toBe(value.defender.total);
    expect(value.attacker.total).toBe(2);
  });
});
