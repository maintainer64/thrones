import { describe, expect, it } from "vitest";
import { parseBid } from "./bid";

describe("sealed bid input", () => {
  it("accepts an explicit zero, including with no power left", () => {
    expect(parseBid("0", 0)).toBe(0);
    expect(parseBid("0", 20)).toBe(0);
  });
  it("accepts integer bids through the current available balance", () => {
    expect(parseBid("13", 20)).toBe(13);
    expect(parseBid("20", 20)).toBe(20);
  });
  it("rejects missing, negative, fractional and non-finite input", () => {
    for (const value of [
      "",
      " ",
      "-1",
      "1.5",
      "1e1",
      "Infinity",
      "NaN",
      "+1",
      "9007199254740993",
    ])
      expect(parseBid(value, 20)).toBeUndefined();
  });
  it("revalidates a draft against a newer server balance", () => {
    expect(parseBid("5", 5)).toBe(5);
    expect(parseBid("5", 4)).toBeUndefined();
  });
});
