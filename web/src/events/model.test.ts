import { afterEach, describe, expect, it } from "vitest";
import {
  WildlingCardID,
  WildlingChoiceKind,
  WesterosCardID,
  WesterosDecision,
} from "../enums";
import { setLanguage } from "../i18n/core";
import {
  cardDescription,
  cardImage,
  cardName,
  choiceTitle,
  decisionName,
  eventText,
  eventsIdentity,
  eventProgress,
  isEventsPhase,
  readableEvent,
  wildlingDescription,
} from "./model";
import ru from "./ru.json";
import en from "./en.json";
import type { Room } from "../types";

afterEach(() => setLanguage("ru"));
describe("Westeros and wildling UI protocol", () => {
  it("has identical RU/EN keys, interpolation parameters, and nonempty English text", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(ru).sort());
    for (const key of Object.keys(ru) as (keyof typeof ru)[]) {
      expect(en[key].trim()).not.toBe("");
      expect(en[key]).not.toMatch(/[А-Яа-яЁё]/);
      expect(en[key].match(/\{p\d+\}/g)?.sort() || []).toEqual(
        ru[key].match(/\{p\d+\}/g)?.sort() || [],
      );
    }
  });
  it("covers every engine card, choice and Westeros decision in both languages", () => {
    for (const language of ["ru", "en"] as const) {
      setLanguage(language);
      for (const id of Object.values(WesterosCardID)) {
        expect(cardName(id).length).toBeGreaterThan(2);
        expect(cardDescription(id).length).toBeGreaterThan(10);
        expect(cardImage(id)).toMatch(
          /^\/local-assets\/events\/.+\.webp\?v=[a-f0-9]{12}$/,
        );
      }
      for (const id of Object.values(WildlingCardID)) {
        expect(cardName(id).length).toBeGreaterThan(2);
        expect(wildlingDescription(id, true).length).toBeGreaterThan(5);
        expect(wildlingDescription(id, false).length).toBeGreaterThan(5);
        expect(cardImage(id)).not.toContain("undefined");
      }
      for (const id of Object.values(WildlingChoiceKind))
        expect(choiceTitle(id)).toBeTruthy();
      for (const id of Object.values(WesterosDecision))
        expect(decisionName(id)).toBeTruthy();
    }
  });
  it("does not reset drafts or reopen dialogs on unrelated revisions or language", () => {
    const room = {
      id: "a",
      revision: 1,
      round: 2,
      phase: "wildling_bidding",
      wildlings: {
        attack: { strength: 6, bids: { submitted: { stark: false } } },
      },
    } as unknown as Room;
    const key = eventsIdentity(room);
    setLanguage("en");
    expect(eventsIdentity({ ...room, revision: 9 })).toBe(key);
    expect(eventsIdentity({ ...room, phase: "wildling_ties" })).not.toBe(key);
    expect(eventsIdentity({ ...room, phase: "planning" })).toBe("");
    expect(isEventsPhase("mustering")).toBe(false);
    expect(isEventsPhase("wildling_effects")).toBe(true);
  });
  it("does not render technical card IDs in the event history", () => {
    setLanguage("en");
    expect(
      readableEvent({
        id: 1,
        type: "wildling-card",
        house: "",
        text: "Wildling card revealed: preemptive-raid",
      }),
    ).toBe("Wildling card revealed: Preemptive Raid");
    expect(eventText("total", { p0: 0, p1: 6 })).toContain("0");
  });
  it("does not call a nested mustering or wildling event resolved too early", () => {
    const room = {
      phase: "mustering",
      mustering: {},
      westeros: { run: { nextCard: 1, stage: "events" } },
    } as unknown as Room;
    expect(eventProgress(room, 0)).toBe("current");
    expect(eventProgress(room, 1)).toBe("pending");
    expect(eventProgress({ ...room, mustering: undefined }, 0)).toBe(
      "resolved",
    );
    expect(
      eventProgress(
        {
          ...room,
          westeros: {
            ...room.westeros!,
            run: { ...room.westeros!.run!, nextCard: 0, stage: "wildlings" },
          },
        },
        0,
      ),
    ).toBe("pending");
    expect(
      eventsIdentity({ id: "a", round: 2, phase: "westeros" } as Room),
    ).toBe("");
  });
});
