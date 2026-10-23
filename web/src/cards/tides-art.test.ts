import { describe, expect, it } from "vitest";
import { TidesCardID } from "../enums";
import { tidesArt, tidesArtURL } from "./tides-art";

describe("user HD Tides faces", () => {
  it("maps all six engine IDs to their versioned PNGs", () => {
    expect(Object.keys(tidesArt).sort()).toEqual(
      Object.values(TidesCardID).sort(),
    );
    expect(
      new Set(Object.values(tidesArt).map((face) => face.revision)).size,
    ).toBe(6);
    for (const id of Object.values(TidesCardID)) {
      expect(tidesArt[id].revision).toMatch(/^[a-f0-9]{12}$/);
      expect(tidesArtURL(id)).toBe(
        `/local-assets/tides-of-battle/${id}.webp?v=${tidesArt[id].revision}`,
      );
    }
  });
  it("retains each original portrait ratio and HD resolution", () => {
    for (const face of Object.values(tidesArt)) {
      expect(face.width).toBeGreaterThanOrEqual(1000);
      expect(face.height).toBeGreaterThanOrEqual(1500);
      expect(face.height / face.width).toBeGreaterThanOrEqual(1.5);
      expect(face.height / face.width).toBeLessThan(1.6);
    }
  });
});
