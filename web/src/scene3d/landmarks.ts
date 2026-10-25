import type { Region } from "../types";
import type { GameManifest, Point2 } from "./game-state";
import { containsTriangle } from "./terrain";
import type { Triangle } from "./terrain";

export type Landmark = {
  region: string;
  kind: "castle" | "stronghold" | "port";
  point: Point2;
  rotation?: number;
};

// A port is a maritime region. Put its model inside that region, facing the
// adjoining land, rather than applying the generic castle offset into open sea.
export function coastalPortPoint(
  port: Region,
  regions: Region[],
  manifest: GameManifest,
  triangles: Record<string, Triangle[]>,
) {
  const faces = triangles[port.id] || [];
  const shore = (port.neighbors || [])
    .filter((id) =>
      regions.some((region) => region.id === id && region.type === "land"),
    )
    .flatMap((id) => triangles[id] || []);
  if (!faces.length || !shore.length) return undefined;
  const anchor = manifest.regions[port.id].position;
  let nearest: Point2 = anchor,
    distance = Infinity;
  for (const triangle of shore)
    for (let i = 0; i < 3; i++) {
      const a = triangle[i],
        b = triangle[(i + 1) % 3],
        dx = b[0] - a[0],
        dz = b[1] - a[1],
        length = dx * dx + dz * dz;
      const t = length
        ? Math.max(
            0,
            Math.min(
              1,
              ((anchor[0] - a[0]) * dx + (anchor[1] - a[1]) * dz) / length,
            ),
          )
        : 0;
      const p: Point2 = [a[0] + dx * t, a[1] + dz * t],
        d = Math.hypot(p[0] - anchor[0], p[1] - anchor[1]);
      if (d < distance) {
        distance = d;
        nearest = p;
      }
    }
  const candidates: Point2[] = faces.map((triangle) => [
    (triangle[0][0] + triangle[1][0] + triangle[2][0]) / 3,
    (triangle[0][1] + triangle[1][1] + triangle[2][1]) / 3,
  ]);
  const safe = candidates.filter(
    ([x, z]) =>
      Math.abs(x) < manifest.board.width / 2 - 0.4 &&
      Math.abs(z) < manifest.board.depth / 2 - 0.4,
  );
  const point = (safe.length ? safe : candidates).reduce((best, p) =>
    Math.hypot(p[0] - nearest[0], p[1] - nearest[1]) <
    Math.hypot(best[0] - nearest[0], best[1] - nearest[1])
      ? p
      : best,
  );
  return {
    point,
    rotation: Math.atan2(nearest[0] - point[0], nearest[1] - point[1]),
  };
}
export function placeLandmarks(
  regions: Region[],
  manifest: GameManifest,
  triangles: Record<string, Triangle[]>,
): Landmark[] {
  return regions
    .filter((region) => region.type === "port" || region.castleLevel > 0)
    .map((region) => {
      const coastal =
        region.type === "port"
          ? coastalPortPoint(region, regions, manifest, triangles)
          : undefined;
      if (coastal)
        return { region: region.id, kind: "port" as const, ...coastal };
      const anchor = manifest.regions[region.id];
      const [x, z] = anchor.position;
      const candidates: Point2[] = [
        [x - 0.5, z + 0.25],
        [x + 0.5, z + 0.25],
        [x - 0.45, z - 0.3],
        [x + 0.45, z - 0.3],
        anchor.label,
        anchor.position,
      ];
      const inside = (p: Point2) =>
        (triangles[region.id] || []).some((triangle) =>
          containsTriangle(p, triangle),
        );
      const point = candidates.find(inside) || anchor.position;
      return {
        region: region.id,
        kind:
          region.type === "port"
            ? "port"
            : region.castleLevel > 1
              ? "stronghold"
              : "castle",
        point,
      };
    });
}
