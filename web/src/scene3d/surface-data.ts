import { BufferGeometry, Float32BufferAttribute, Mesh, Vector3 } from "three";
import type { Object3D } from "three";
import type { Region } from "../types";
import type { GameManifest } from "./game-state";
import {
  boundaryEdges,
  createRelief,
  subdivideTriangle,
  clipTriangle,
} from "./terrain";
import type { Triangle, Ridge, XZ } from "./terrain";

export function createSurfaceData(
  scene: Object3D,
  manifest: GameManifest,
  regions: Region[],
) {
  scene.updateMatrixWorld(true);
  const triangles: Record<string, Triangle[]> = {};
  scene.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    let parent: Object3D | null = object;
    while (parent && !parent.userData.regionId) parent = parent.parent;
    const id = parent?.userData.regionId as string | undefined;
    if (!id) return;
    const geometry = object.geometry,
      attribute = geometry.getAttribute("position"),
      index = geometry.index;
    const count = index?.count || attribute.count;
    const point = (index: number): XZ => {
      const value = new Vector3()
        .fromBufferAttribute(attribute, index)
        .applyMatrix4(object.matrixWorld);
      return [value.x, value.z];
    };
    const list = (triangles[id] ??= []);
    for (let i = 0; i < count; i += 3)
      list.push(
        ...clipTriangle(
          [
            point(index ? index.getX(i) : i),
            point(index ? index.getX(i + 1) : i + 1),
            point(index ? index.getX(i + 2) : i + 2),
          ],
          manifest.board.width,
          manifest.board.depth,
        ),
      );
  });
  const land = regions
    .filter((region) => region.type === "land")
    .flatMap((region) => triangles[region.id] || []);
  const ports = regions
    .filter((region) => region.type === "port")
    .flatMap((region) => triangles[region.id] || []);
  const ridge = (
    id: string,
    height: number,
    width: number,
    length: number,
    ox = 0,
    oz = 0,
    angle = 0,
  ): Ridge => {
    const p = manifest.regions[id].label;
    return { x: p[0] + ox, z: p[1] + oz, height, width, length, angle };
  };
  const relief = createRelief(
    manifest.board.width,
    manifest.board.depth,
    land,
    ports,
    [
      ridge("the-mountains-of-the-moon", 0.58, 0.58, 1.25, -0.12, 0.05, -0.35),
      ridge("the-eyrie", 0.4, 0.45, 0.65, 0.1, 0.12),
      ridge("princes-pass", 0.48, 1.1, 0.4, 0, 0.15, 0.2),
      ridge("the-boneway", 0.38, 0.5, 0.8, 0.05, 0.05, -0.5),
      ridge("winterfell", 0.32, 0.5, 1.1, 0.9, 0.5, -0.2),
      ridge("castle-black", 0.4, 0.65, 0.65, 1, -0.45),
      ridge("lannisport", 0.27, 0.5, 0.8, -0.35, 0.4, 0.1),
      ridge("searoad-marches", 0.25, 0.45, 0.9, -0.1, 0.2),
      ridge("karhold", 0.2, 0.65, 0.75, 0.4, 0.3),
      ridge("dragonstone", 0.28, 0.45, 0.6, -0.12, -0.1),
    ],
  );
  const surfaces: Record<string, BufferGeometry> = {},
    borders: Record<string, [number, number, number][]> = {};
  for (const [id, faces] of Object.entries(triangles)) {
    const positions: number[] = [];
    for (const triangle of faces)
      subdivideTriangle(triangle, ([x, z]) =>
        positions.push(x, relief.height(x, z) + 0.018, z),
      );
    surfaces[id] = new BufferGeometry().setAttribute(
      "position",
      new Float32BufferAttribute(positions, 3),
    );
    const lines: [number, number, number][] = [];
    for (const [a, b] of boundaryEdges(faces)) {
      const steps = Math.max(
        1,
        Math.ceil(Math.hypot(a[0] - b[0], a[1] - b[1]) / 0.1),
      );
      for (let step = 0; step < steps; step++)
        for (const t of [step / steps, (step + 1) / steps]) {
          const x = a[0] + (b[0] - a[0]) * t,
            z = a[1] + (b[1] - a[1]) * t;
          lines.push([x, relief.height(x, z) + 0.035, z]);
        }
    }
    borders[id] = lines;
  }
  return { relief, surfaces, borders, triangles };
}

export type SurfaceData = ReturnType<typeof createSurfaceData>;

export function terrainPlane(
  width: number,
  minZ: number,
  maxZ: number,
  height: (x: number, z: number) => number,
  segments = 144,
) {
  const positions: number[] = [],
    uv: number[] = [],
    indices: number[] = [];
  for (let row = 0; row <= segments; row++)
    for (let col = 0; col <= segments; col++) {
      const u = col / segments,
        v = row / segments,
        x = (u - 0.5) * width,
        z = minZ + v * (maxZ - minZ);
      positions.push(x, height(x, z), z);
      uv.push(1 - u, 1 - v);
    }
  for (let row = 0; row < segments; row++)
    for (let col = 0; col < segments; col++) {
      const a = row * (segments + 1) + col,
        b = a + 1,
        c = a + segments + 1,
        d = c + 1;
      indices.push(a, c, b, b, c, d);
    }
  const geometry = new BufferGeometry();
  geometry.setAttribute("position", new Float32BufferAttribute(positions, 3));
  geometry.setAttribute("uv", new Float32BufferAttribute(uv, 2));
  geometry.setIndex(indices);
  geometry.computeVertexNormals();
  return geometry;
}
