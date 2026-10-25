import { describe, expect, it } from "vitest";
import { phaseStep } from "./PhaseSteps";
describe("phase ribbon", () => {
  it("groups real phases without inventing event progress", () => {
    expect(phaseStep("lobby")).toBe(-1);
    expect(phaseStep("planning")).toBe(0);
    expect(phaseStep("orders_revealed")).toBe(1);
    expect(phaseStep("raven")).toBe(1);
    for (const phase of [
      "raids",
      "marches",
      "combat",
      "retreat",
      "card_effect",
      "consolidate_power",
    ])
      expect(phaseStep(phase)).toBe(2);
    for (const phase of [
      "westeros",
      "supply",
      "mustering",
      "influence_bidding",
      "influence_ties",
      "wildling_bidding",
      "wildling_ties",
      "wildling_effects",
      "finished",
    ])
      expect(phaseStep(phase)).toBe(3);
  });
});
