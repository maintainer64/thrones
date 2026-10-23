import { describe, it, expect } from "vitest";
import {
  boundaryEdges,
  containsTriangle,
  createRelief,
  subdivideTriangle,
  clipTriangle,
} from "./terrain";
import type { Triangle } from "./terrain";

const island: Triangle[] = [
  [
    [-2, -2],
    [2, -2],
    [2, 2],
  ],
  [
    [-2, -2],
    [2, 2],
    [-2, 2],
  ],
];
describe("relief registered to original region colliders", () => {
  it("clips extended sea colliders to the table and discards off-map triangles", () => {
    const pieces = clipTriangle(
      [
        [-30, -30],
        [30, -30],
        [0, 30],
      ],
      10,
      20,
    );
    expect(pieces.length).toBeGreaterThan(0);
    for (const point of pieces.flat()) {
      expect(Math.abs(point[0])).toBeLessThanOrEqual(5.000001);
      expect(Math.abs(point[1])).toBeLessThanOrEqual(10.000001);
    }
    expect(
      clipTriangle(
        [
          [20, 20],
          [30, 20],
          [25, 30],
        ],
        10,
        20,
      ),
    ).toHaveLength(0);
  });
  it("excludes degenerate triangles and accepts either winding", () => {
    expect(containsTriangle([0, 0], island[0])).toBe(true);
    expect(containsTriangle([0, 0], [...island[0]].reverse() as Triangle)).toBe(
      true,
    );
    expect(
      containsTriangle(
        [0, 0],
        [
          [0, 0],
          [1, 0],
          [2, 0],
        ],
      ),
    ).toBe(false);
    expect(containsTriangle([5, 0], island[0])).toBe(false);
  });
  it("leaves the sea flat and raises the land, with mountains above foothills", () => {
    const field = createRelief(
      10,
      10,
      island,
      [],
      [{ x: 0, z: 0, height: 0.5, width: 0.6, length: 1 }],
      64,
      64,
    );
    expect(field.height(4, 4)).toBe(0);
    expect(field.height(0, 0)).toBeGreaterThan(0.5);
    expect(field.height(1.5, 1.5)).toBeGreaterThan(0.05);
    expect(field.height(0, 0)).toBeGreaterThan(field.height(1.5, 1.5));
    expect([...field.values].every(Number.isFinite)).toBe(true);
  });
  it("carves ports out of the raised land and samples safely outside the map", () => {
    const port: Triangle[] = [
      [
        [-0.5, -0.5],
        [0.5, -0.5],
        [0, 0.5],
      ],
    ];
    const field = createRelief(10, 10, island, port, [], 64, 64);
    expect(field.height(0, 0)).toBe(0);
    expect(field.height(-500, 500)).toBe(0);
  });
  it("does not draw triangulation diagonals as territory borders", () => {
    expect(boundaryEdges(island)).toHaveLength(4);
  });
  it("subdivides picking surfaces without moving their XZ footprint", () => {
    const vertices: [number, number][] = [];
    subdivideTriangle(island[0], (p) => vertices.push(p), 0.5);
    expect(vertices.length).toBeGreaterThan(3);
    expect(vertices.length % 3).toBe(0);
    for (const point of vertices)
      expect(containsTriangle(point, island[0])).toBe(true);
  });
});
