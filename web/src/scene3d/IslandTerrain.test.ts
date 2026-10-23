import { expect, it } from "vitest";
import { BufferGeometry, Float32BufferAttribute } from "three";
import { islandGeometry } from "./IslandTerrain";
it("gives land its own stone surface without changing picking contours or adding sea", () => {
  const source = new BufferGeometry().setAttribute(
    "position",
    new Float32BufferAttribute([0, 0.2, 0, 1, 0.3, 0, 0, 0.2, 1], 3),
  );
  const original = Array.from(source.getAttribute("position").array);
  const geometry = islandGeometry(source)!;
  expect(geometry).not.toBe(source);
  const positions = geometry.getAttribute("position"),
    colors = geometry.getAttribute("color");
  for (let i = 0; i < 3; i++) {
    expect(positions.getX(i)).toBe(original[i * 3]);
    expect(positions.getZ(i)).toBe(original[i * 3 + 2]);
    expect(positions.getY(i)).toBeCloseTo(original[i * 3 + 1] - 0.006, 5);
    expect(colors.getX(i)).toBeGreaterThan(colors.getZ(i));
    expect(colors.getY(i)).toBeGreaterThan(colors.getZ(i));
  }
  expect(Array.from(source.getAttribute("position").array)).toEqual(original);
  expect(source.getAttribute("color")).toBeUndefined();
  geometry.dispose();
  source.dispose();
  expect(islandGeometry()).toBeUndefined();
});
