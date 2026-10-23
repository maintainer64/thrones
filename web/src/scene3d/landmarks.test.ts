import { describe, expect, it } from "vitest";
import { coastalPortPoint, placeLandmarks } from "./landmarks";
import { containsTriangle } from "./terrain";
import type { GameManifest } from "./game-state";
import type { Region } from "../types";
import type { Triangle } from "./terrain";
describe("architectural landmarks", () => {
  const polygon: Triangle = [
    [-2, -2],
    [2, -2],
    [0, 2],
  ];
  const regions = [
    { id: "city", type: "land", castleLevel: 1 },
    { id: "capital", type: "land", castleLevel: 2 },
    { id: "harbour", type: "port", castleLevel: 0 },
    { id: "sea", type: "sea", castleLevel: 0 },
  ] as Region[];
  const manifest = {
    regions: Object.fromEntries(
      regions.map((region) => [region.id, { position: [0, 0], label: [0, 0] }]),
    ),
  } as unknown as GameManifest;
  it("uses port models only for ports, distinct castle and stronghold models", () => {
    const result = placeLandmarks(regions, manifest, {
      city: [polygon],
      capital: [polygon],
      harbour: [polygon],
    });
    expect(result.map((marker) => marker.kind)).toEqual([
      "castle",
      "stronghold",
      "port",
    ]);
    for (const marker of result) {
      expect(containsTriangle(marker.point, polygon)).toBe(true);
      expect(Math.hypot(...marker.point)).toBeGreaterThan(0.4);
    }
  });
  it("falls back to the original region anchor for tiny regions", () => {
    const result = placeLandmarks(regions.slice(0, 1), manifest, {});
    expect(result[0].point).toEqual([0, 0]);
  });
  it("places a port inside its maritime region, close to its own land and facing the shore", () => {
    const coast: Triangle[] = [
      [
        [0, 0],
        [2, 0],
        [0, 2],
      ],
    ];
    const water: Triangle[] = [
      [
        [0, -2],
        [2, -2],
        [2, -0.1],
      ],
      [
        [0, -2],
        [2, -0.1],
        [0, -0.1],
      ],
    ];
    const port = { id: "port", type: "port", neighbors: ["island"] } as Region;
    const land = { id: "island", type: "land" } as Region;
    const manifest = {
      board: { width: 10, depth: 20 },
      regions: { port: { position: [1, -0.05] } },
    } as unknown as GameManifest;
    const result = coastalPortPoint(port, [port, land], manifest, {
      port: water,
      island: coast,
    })!;
    expect(water.some((face) => containsTriangle(result.point, face))).toBe(
      true,
    );
    expect(result.point[1]).toBeLessThan(0);
    expect(result.point[1]).toBeGreaterThan(-1);
    expect(Math.cos(result.rotation)).toBeGreaterThan(0);
    expect(Math.abs(result.point[0])).toBeLessThan(4.6);
  });
});
