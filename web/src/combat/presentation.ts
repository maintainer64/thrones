import { useEffect, useMemo, useState } from "react";
import type { CombatRecord, Room } from "../types";
import { ReplayPhase, ReportStage, type CombatPlayback } from "./animation";

export type CombatStep = { record: CombatRecord; stage: "intro" | "result" };
export const stepID = (step: CombatStep) => `${step.record.id}:${step.stage}`;
export const storageKey = (roomID: string) =>
  `westeros-combat-read-v1:${roomID}`;

export function hasCombatDecision(room?: Room) {
  const b = room?.battle;
  return (
    !!room &&
    !!b &&
    ((room.phase === "combat" &&
      [b.attacker, b.defender].includes(room.yourHouse) &&
      !b.yourCard) ||
      (room.phase === "card_effect" && b.choice?.house === room.yourHouse) ||
      (room.phase === "retreat" &&
        (b.retreatChooser || b.loser) === room.yourHouse))
  );
}

export function nextCombatStep(
  room: Room | undefined,
  read: string[],
): CombatStep | undefined {
  const records = room?.match?.combats || [];
  if (hasCombatDecision(room)) {
    // Never bury a mandatory live choice beneath old bot combat reports.
    const record = records.find((record) => record.id === room?.battle?.id);
    if (record && !read.includes(`${record.id}:intro`))
      return { record, stage: "intro" };
    return;
  }
  for (const record of records) {
    if (!read.includes(`${record.id}:intro`)) return { record, stage: "intro" };
    if (record.result && !read.includes(`${record.id}:result`))
      return { record, stage: "result" };
  }
}

function loadRead(roomID: string): string[] {
  try {
    const value: unknown = JSON.parse(
      localStorage.getItem(storageKey(roomID)) || "[]",
    );
    return Array.isArray(value)
      ? value.filter((id): id is string => typeof id === "string").slice(-80)
      : [];
  } catch {
    return [];
  }
}
type PresentationState = {
  roomID: string;
  read: string[];
  active?: CombatStep;
  onMap?: boolean;
  deferred?: string;
  replay?: boolean;
  playback?: CombatPlayback;
  serial?: number;
  approachViewed?: number;
  reportFor?: number;
  reportStage?: ReportStage;
};

export function useCombatPresentation(room?: Room) {
  const initial = useMemo<PresentationState>(
    () => ({ roomID: room?.id || "", read: loadRead(room?.id || "") }),
    [room?.id],
  );
  const [stored, setStored] = useState<PresentationState>(initial);
  const state = stored.roomID === initial.roomID ? stored : initial;
  const decision = hasCombatDecision(room)
    ? `${room?.battle?.id}:${room?.phase}:${room?.battle?.choice?.kind}:${room?.battle?.yourCard}`
    : undefined;
  const deferred = !!decision && decision === state.deferred;
  const candidate = deferred ? undefined : nextCombatStep(room, state.read);
  const pinned = deferred ? undefined : state.active || candidate;
  const active = pinned
    ? {
        ...pinned,
        record:
          room?.match?.combats?.find(
            (record) => record.id === pinned.record.id,
          ) || pinned.record,
      }
    : undefined;
  // Pin an immutable public step even if bots finish this battle while it is
  // being read. Subsequent steps are recovered from persisted server history.
  useEffect(() => {
    setStored((previous) => {
      const current = previous.roomID === initial.roomID ? previous : initial;
      if (current.active || !candidate) return current;
      return { ...current, active: candidate };
    });
  }, [initial, candidate?.record.id, candidate?.stage]);
  const update = (change: (value: PresentationState) => PresentationState) => {
    setStored((previous) => {
      const next = change(
        previous.roomID === initial.roomID ? previous : initial,
      );
      try {
        localStorage.setItem(
          storageKey(next.roomID),
          JSON.stringify(next.read),
        );
      } catch {
        /* Storage may be disabled. */
      }
      return next;
    });
  };
  return {
    active,
    visible: !state.onMap && !!active,
    onMap: !!state.onMap,
    pending: !!active,
    playback: state.playback,
    approachViewed: state.approachViewed === active?.record.id,
    reportStage:
      state.reportFor === active?.record.id
        ? state.reportStage || ReportStage.Cards
        : ReportStage.Cards,
    setReportStage: (stage: ReportStage) =>
      update((value) => ({
        ...value,
        reportFor: active?.record.id,
        reportStage: stage,
      })),
    next: () =>
      update((value) => ({
        ...value,
        active:
          value.replay && active?.stage === "intro" && active.record.result
            ? { record: active.record, stage: "result" }
            : undefined,
        replay:
          value.replay && active?.stage === "intro" && !!active.record.result,
        onMap: false,
        playback: undefined,
        read: active
          ? [...new Set([...value.read, stepID(active)])].slice(-80)
          : value.read,
      })),
    show: (record: CombatRecord, stage: CombatStep["stage"] = "intro") =>
      update((value) => ({
        ...value,
        active: { record, stage },
        replay: true,
        onMap: false,
        playback: undefined,
        approachViewed: undefined,
        reportFor: record.id,
        reportStage: ReportStage.Cards,
      })),
    showMap: () =>
      update((value) => {
        if (!active) return value;
        const phase =
          active.stage === "intro" ? ReplayPhase.Approach : ReplayPhase.Losses;
        const serial = (value.serial || 0) + 1;
        return {
          ...value,
          active,
          onMap: true,
          serial,
          playback: {
            record: active.record,
            phase,
            serial,
            playing: true,
            complete: false,
          },
          reportFor: active.record.id,
          reportStage:
            phase === ReplayPhase.Losses
              ? ReportStage.Outcome
              : value.reportStage,
        };
      }),
    skipPlayback: () =>
      update((value) => ({
        ...value,
        playback: value.playback
          ? { ...value.playback, complete: true, playing: false }
          : undefined,
      })),
    togglePlayback: () =>
      update((value) => ({
        ...value,
        playback:
          value.playback && !value.playback.complete
            ? { ...value.playback, playing: !value.playback.playing }
            : value.playback,
      })),
    completePlayback: (serial: number) =>
      update((value) =>
        value.playback?.serial === serial
          ? {
              ...value,
              playback: { ...value.playback, complete: true, playing: false },
            }
          : value,
      ),
    resume: () =>
      update((value) => ({
        ...value,
        onMap: false,
        approachViewed:
          value.playback?.phase === ReplayPhase.Approach
            ? value.playback.record.id
            : value.approachViewed,
        playback: undefined,
      })),
    toDecision: () =>
      update((value) => ({
        ...value,
        active: undefined,
        onMap: false,
        playback: undefined,
        deferred: decision,
        read: room?.battle?.id
          ? [...new Set([...value.read, `${room.battle.id}:intro`])].slice(-80)
          : value.read,
      })),
  };
}
