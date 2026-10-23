import { expect, it } from "vitest";
import { terrainPlane } from "./surface-data";

it("builds raised terrain geometry with upward normals and original map UV orientation", () => {
  const height = (x: number, z: number) => 0.2 + x * 0.05 + z * 0.02;
  const geometry = terrainPlane(2, -2, 0, height, 8);
  const positions = geometry.getAttribute("position"),
    normals = geometry.getAttribute("normal"),
    uv = geometry.getAttribute("uv");
  expect(positions.count).toBe(81);
  for (let i = 0; i < positions.count; i++) {
    expect(positions.getY(i)).toBeCloseTo(
      height(positions.getX(i), positions.getZ(i)),
      5,
    );
    expect(normals.getY(i)).toBeGreaterThan(0.9);
  }
  expect([uv.getX(0), uv.getY(0)]).toEqual([1, 1]);
  expect([uv.getX(80), uv.getY(80)]).toEqual([0, 0]);
  geometry.dispose();
});
