import { describe, expect, it } from "vitest";
import {
  cardRulesTop,
  fittedCardWidth,
  pointerPose,
  textureKey,
} from "./card-art";
describe("house card presentation", () => {
  it("fits the whole card by height as well as width", () => {
    for (const [width, height] of [
      [500, 640],
      [1024, 380],
      [230, 450],
      [400, 260],
    ]) {
      const card = fittedCardWidth(width, height);
      expect(card).toBeLessThanOrEqual(width - 14);
      expect(card * 1.7).toBeLessThanOrEqual(height - 14);
    }
    expect(fittedCardWidth(2000, 2000)).toBe(440);
    expect(fittedCardWidth(0, 0)).toBe(1);
  });
  it("makes room for longer abilities without shrinking their type", () => {
    expect(cardRulesTop("x".repeat(100))).toBe(66);
    expect(cardRulesTop("x".repeat(180))).toBe(60);
    expect(cardRulesTop("x".repeat(300))).toBe(54);
  });
  it("maps base cards to their original portraits", () => {
    expect(textureKey("eddard-stark", "stark")).toBe("T_crd_stark_eddard");
    expect(textureKey("ser-rodrick-cassel", "stark")).toBe(
      "T_crd_stark_rodrik",
    );
    expect(textureKey("the-red-viper", "martell")).toBe(
      "T_crd_martell_theredviper",
    );
    expect(textureKey("queen-of-thorns", "tyrell")).toBe(
      "T_crd_tyrell_queenofthorns",
    );
    expect(textureKey("the-hound", "lannister")).toBe("T_crd_lannister_sandor");
    expect(textureKey("dagmar-cleftjaw", "greyjoy")).toBe(
      "T_crd_greyjoy_dagmer",
    );
  });
  it("keeps perspective bounded even outside the card", () => {
    expect(pointerPose(0.5, 0.5)).toEqual({ x: -0, y: 0, light: 50 });
    expect(pointerPose(-10, 20)).toEqual({ x: -5, y: -6, light: 0 });
    expect(pointerPose(10, -20)).toEqual({ x: 5, y: 6, light: 100 });
  });
});
