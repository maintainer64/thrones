import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  AuctionStage,
  CardChoiceKind,
  InfluenceTrack,
  TidesCardID,
  TidesDecision,
  UnitType,
  WildlingCardID,
  WildlingChoiceKind,
  WesterosCardID,
  WesterosDecision,
  RavenStage,
  RavenDecision,
  FortificationKind,
  CombatLineKind,
} from "./enums";

describe("typed combat protocol values", () => {
  for (const [values, goFile, goType] of [
    [TidesCardID, "tides.go", "TidesCardID"],
    [TidesDecision, "tides.go", "TidesDecision"],
    [CardChoiceKind, "card_effects.go", "CardChoiceKind"],
    [InfluenceTrack, "influence.go", "InfluenceTrack"],
    [AuctionStage, "influence.go", "AuctionStage"],
    [UnitType, "provision.go", "UnitType"],
    [WildlingCardID, "wildlings.go", "WildlingCardID"],
    [WildlingChoiceKind, "wildling_effects.go", "WildlingChoiceKind"],
    [WesterosCardID, "westeros.go", "WesterosCardID"],
    [WesterosDecision, "westeros_phase.go", "WesterosDecision"],
    [RavenStage, "raven.go", "RavenStage"],
    [RavenDecision, "raven.go", "RavenDecision"],
    [FortificationKind, "setup.go", "FortificationKind"],
    [CombatLineKind, "combat_presentation.go", "CombatLineKind"],
  ]) {
    it(`matches Go ${goType} without translated IDs`, () => {
      const source = readFileSync(
        new URL(`../../internal/game/${goFile}`, import.meta.url),
        "utf8",
      );
      const ids = [
        ...source.matchAll(
          new RegExp(`\\w+\\s+${goType}\\s*=\\s*"([^"]+)"`, "g"),
        ),
      ]
        .map((match) => match[1])
        .sort();
      expect(Object.values(values).sort()).toEqual(ids);
    });
  }
});
