import { CombatLineKind, FortificationKind } from "../enums";
import type { CombatRecord, CombatStrength, Room, Unit } from "../types";
import type { Piece } from "../scene3d/game-state";

export enum ReplayPhase {
  Approach = "approach",
  Losses = "losses",
}
export enum ReportStage {
  Cards = "cards",
  Strength = "strength",
  Outcome = "outcome",
}
export type ReplayActor = {
  piece: Piece;
  origin: string;
  destination: string;
  role: "attacker" | "defender" | "remaining" | "support";
  lost: boolean;
  retreatTo?: string;
};
export type CombatPlayback = {
  record: CombatRecord;
  phase: ReplayPhase;
  serial: number;
  playing: boolean;
  complete: boolean;
};

/** Presentation only: quantities, losses and destinations are saved server data. */
export function replayActors(record: CombatRecord): ReplayActor[] {
  const actors: ReplayActor[] = [];
  const assignedLosses = new Map<string, number>();
  let id = 1;
  const add = (
    units: Unit[],
    role: ReplayActor["role"],
    origin: string,
    destination: string,
  ) => {
    for (const unit of units) {
      const lost =
        role === "attacker" || role === "defender"
          ? (record.result?.losses?.[unit.house] || [])
              .filter((loss) => loss.unitType === unit.unitType)
              .reduce((sum, loss) => sum + loss.quantity, 0)
          : 0;
      const key = `${unit.house}:${unit.unitType}`;
      const offset = assignedLosses.get(key) || 0;
      const lossesHere = Math.min(unit.quantity, Math.max(0, lost - offset));
      if (role === "attacker" || role === "defender")
        assignedLosses.set(key, offset + lossesHere);
      for (let slot = 0; slot < unit.quantity; slot++) {
        actors.push({
          piece: {
            id: id++,
            house: unit.house,
            type: unit.unitType,
            slot,
            region: destination,
            battle: role === "attacker",
          },
          origin,
          destination,
          role,
          lost: slot < lossesHere,
          retreatTo:
            (role === "attacker" || role === "defender") &&
            !(slot < lossesHere) &&
            record.result?.retreatHouse === unit.house
              ? record.result.retreatTo
              : role === "attacker" && !(slot < lossesHere)
                ? record.result?.returnTo
                : undefined,
        });
      }
    }
  };
  add(record.attackers, "attacker", record.from, record.to);
  add(record.defenders, "defender", record.to, record.to);
  add(record.remaining, "remaining", record.from, record.from);
  for (const side of [record.initial.attacker, record.initial.defender]) {
    for (const line of side.lines) {
      if (
        line.kind === CombatLineKind.SupportUnit &&
        line.region &&
        line.unit &&
        line.count
      )
        add(
          [{ house: side.house, unitType: line.unit, quantity: line.count }],
          "support",
          line.region,
          line.region,
        );
    }
  }
  return actors;
}
const clamp = (value: number) => Math.max(0, Math.min(1, value));
const smooth = (value: number) => {
  const p = clamp(value);
  return p * p * (3 - 2 * p);
};
export function replayPose(
  actor: ReplayActor,
  phase: ReplayPhase,
  progress: number,
) {
  const p = clamp(progress);
  return {
    travel:
      actor.role === "attacker"
        ? phase === ReplayPhase.Approach
          ? smooth((p - 0.25) / 0.75)
          : 1
        : 0,
    fade:
      phase === ReplayPhase.Losses && actor.lost
        ? smooth((p - 0.15) / 0.35)
        : 0,
    retreat:
      phase === ReplayPhase.Losses && actor.retreatTo
        ? smooth((p - 0.5) / 0.5)
        : 0,
  };
}
export function replayPieces(
  record: CombatRecord,
  phase: ReplayPhase,
  complete: boolean,
) {
  if (phase === ReplayPhase.Losses && complete && record.result?.final) {
    let id = 1;
    return Object.entries(record.result.final.armies).flatMap(
      ([region, units]) =>
        units.flatMap((unit) =>
          Array.from({ length: unit.quantity }, (_, slot) => ({
            id: id++,
            region,
            house: unit.house,
            type: unit.unitType,
            slot,
            battle: false,
          })),
        ),
    );
  }
  return replayActors(record)
    .filter(
      (actor) => !(phase === ReplayPhase.Losses && complete && actor.lost),
    )
    .map((actor) => ({
      ...actor.piece,
      region:
        phase === ReplayPhase.Approach && !complete
          ? actor.origin
          : phase === ReplayPhase.Losses && complete && actor.retreatTo
            ? actor.retreatTo
            : actor.destination,
    }));
}
export const replayDuration = (phase: ReplayPhase) =>
  phase === ReplayPhase.Approach ? 4 : 4.5;
export function advanceReplay(
  progress: number,
  elapsedSeconds: number,
  phase: ReplayPhase,
) {
  return clamp(progress + Math.max(0, elapsedSeconds) / replayDuration(phase));
}
/** Never replace live room state: this facade is used by the 3D renderer only. */
export function replayRoom(room: Room, playback: CombatPlayback): Room {
  const { record, phase, complete } = playback;
  const armies: Record<string, Unit[]> = {};
  for (const piece of replayPieces(record, phase, complete)) {
    const army = (armies[piece.region] ||= []);
    const unit = army.find(
      (unit) => unit.house === piece.house && unit.unitType === piece.type,
    );
    if (unit) unit.quantity++;
    else army.push({ house: piece.house, unitType: piece.type, quantity: 1 });
  }
  const strength = record.initial.defender.lines
    .filter((line) => line.kind === CombatLineKind.Garrison)
    .reduce((sum, line) => sum + line.value, 0);
  const resolved = phase === ReplayPhase.Losses && complete;
  const final = resolved ? record.result?.final : undefined;
  return {
    ...room,
    round: record.round,
    battle: undefined,
    events: [],
    orders: {},
    legalRegions: [],
    fortifications: final
      ? Object.fromEntries(
          Object.entries(final.garrisons).map(([region, value]) => [
            region,
            {
              kind: FortificationKind.Garrison,
              strength: value,
              house: final.control[region] || record.defender,
            },
          ]),
        )
      : strength && !(resolved && record.result?.winner === record.attacker)
        ? {
            [record.to]: {
              kind: FortificationKind.Garrison,
              strength,
              house: record.defender,
            },
          }
        : {},
    match: {
      ...room.match!,
      armies,
      activeHouse: "",
      control: final
        ? { ...final.control }
        : {
            [record.from]: record.attacker,
            [record.to]: resolved
              ? record.result?.winner || record.defender
              : record.defender,
          },
    },
  };
}
export function strengthStepCount(value: CombatStrength) {
  return Math.max(value.attacker.lines.length, value.defender.lines.length);
}
/** Sum provided contributions; no combat rules are duplicated in the client. */
export function strengthAtStep(
  value: CombatStrength,
  count: number,
): CombatStrength {
  const side = (source: CombatStrength["attacker"]) => ({
    ...source,
    total: source.lines
      .slice(0, Math.max(0, count))
      .reduce((sum, line) => sum + line.value, 0),
  });
  return {
    ...value,
    attacker: side(value.attacker),
    defender: side(value.defender),
  };
}
