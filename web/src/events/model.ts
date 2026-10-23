import {
  WildlingCardID,
  WildlingChoiceKind,
  WesterosCardID,
  WesterosDecision,
} from "../enums";
import { getLanguage } from "../i18n/core";
import ru from "./ru.json";
import en from "./en.json";
import type { Room, Event } from "../types";
import art from "./art.json";
import { artworkURL } from "../artwork/artwork";

export const finishReward = "finish";
export type EventKey = keyof typeof ru;
export function eventText(key: EventKey, params: Record<string, unknown> = {}) {
  const dictionary: Record<EventKey, string> = getLanguage() === "en" ? en : ru;
  return dictionary[key].replace(/\{(p\d+)\}/g, (match, field: string) =>
    field in params ? String(params[field]) : match,
  );
}
export const isEventsPhase = (phase: string) =>
  [
    "westeros",
    "wildling_bidding",
    "wildling_ties",
    "wildling_effects",
  ].includes(phase);
export const cardName = (id: WildlingCardID | WesterosCardID) =>
  eventText(`cards.${id}.name`);
export const cardDescription = (id: WesterosCardID) =>
  eventText(`cards.${id}.description`);
export const wildlingDescription = (id: WildlingCardID, won: boolean) =>
  eventText(`cards.${id}.${won ? "won" : "lost"}`);
export const cardImage = (id: WildlingCardID | WesterosCardID) =>
  artworkURL(`events/${art[id]}`);
export const decisionName = (id: WesterosDecision) =>
  eventText(`decisions.${id}`);
export const choiceTitle = (kind: WildlingChoiceKind) =>
  eventText(`choices.${kind}`);

export function eventsIdentity(room: Room) {
  if (!isEventsPhase(room.phase)) return "";
  if (
    !room.westeros?.run &&
    !room.wildlings?.attack &&
    room.events?.at(-1)?.type !== "rules-error"
  )
    return "";
  const attack = room.wildlings?.attack;
  const choice = attack?.choice;
  const event = room.westeros?.run?.choice;
  return [
    room.id,
    room.round,
    room.phase,
    room.wildlings?.last?.card,
    attack?.strength,
    attack?.card,
    attack?.outcome?.selectedHouse,
    choice?.house,
    choice?.kind,
    choice?.count,
    choice?.options?.join(","),
    event?.card,
    event?.house,
  ].join(":");
}

export function eventProgress(room: Room, index: number) {
  const run = room.westeros?.run;
  if (!run) return "pending";
  if (run.stage === "complete") return "resolved";
  const nested =
    !!room.influence ||
    !!room.supply ||
    !!room.mustering ||
    run.stage === "choice" ||
    run.stage === "wildlings";
  const active = run.nextCard - (nested ? 1 : 0);
  return index < active ? "resolved" : index === active ? "current" : "pending";
}

export function readableEvent(event: Event) {
  let text = event.text;
  for (const id of [
    ...Object.values(WildlingCardID),
    ...Object.values(WesterosCardID),
  ].sort((a, b) => b.length - a.length)) {
    text = text.replaceAll(id, cardName(id));
  }
  return text;
}
